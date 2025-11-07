package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// OAuthClient handles OAuth flow with third-party providers
type OAuthClient struct {
	config        *ThirdPartyOAuthConfig
	mcpTokenGen   *TokenGenerator
	store         Store
	stateStore    map[string]*OAuthState
	httpClient    *http.Client
	encryptionKey []byte
	stateMutex    sync.RWMutex
	encryptMutex  sync.RWMutex
}

// OAuthState stores temporary OAuth flow state
type OAuthState struct {
	State           string
	CodeVerifier    string
	CodeChallenge   string
	RedirectURI     string
	OriginalRequest string
	CreatedAt       time.Time
	ExpiresAt       time.Time

	// MCP Client parameters
	MCPClientID            string
	MCPRedirectURI         string
	MCPScopes              []string
	MCPResources           []string
	MCPState               string
	MCPCodeChallenge       string
	MCPCodeChallengeMethod string
}

// NewOAuthClient creates a new OAuth client for third-party auth
func NewOAuthClient(
	config *ThirdPartyOAuthConfig,
	mcpTokenGen *TokenGenerator,
	store Store,
) (*OAuthClient, error) {
	// Generate encryption key for sensitive data
	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	return &OAuthClient{
		config:        config,
		mcpTokenGen:   mcpTokenGen,
		store:         store,
		stateStore:    make(map[string]*OAuthState),
		httpClient:    &http.Client{Timeout: 30 * time.Second},
		encryptionKey: encryptionKey,
	}, nil
}

// InitiateOAuthFlow handles the initial OAuth request from MCP Client
func (c *OAuthClient) InitiateOAuthFlow(w http.ResponseWriter, r *http.Request) {
	// Extract MCP client parameters
	mcpClientID := r.URL.Query().Get("client_id")
	mcpRedirectURI := r.URL.Query().Get("redirect_uri")
	mcpState := r.URL.Query().Get("state")
	mcpScope := r.URL.Query().Get("scope")
	mcpCodeChallenge := r.URL.Query().Get("code_challenge")
	mcpCodeChallengeMethod := r.URL.Query().Get("code_challenge_method")
	mcpResources := r.URL.Query()["resource"]

	// Validate required MCP parameters
	if mcpClientID == "" {
		http.Error(w, "Missing client_id parameter", http.StatusBadRequest)
		return
	}
	if mcpRedirectURI == "" {
		http.Error(w, "Missing redirect_uri parameter", http.StatusBadRequest)
		return
	}

	// Validate MCP client exists
	client, err := c.store.GetClient(r.Context(), mcpClientID)
	if err != nil {
		http.Error(w, "Invalid client_id", http.StatusBadRequest)
		return
	}

	// Validate redirect URI (exact match per MCP spec)
	validRedirect := false
	for _, allowed := range client.RedirectURIs {
		if mcpRedirectURI == allowed {
			validRedirect = true
			break
		}
	}
	if !validRedirect {
		http.Error(w, "Invalid redirect_uri", http.StatusBadRequest)
		return
	}

	// Parse scopes
	mcpScopes := parseScopes(mcpScope)

	// Generate state for CSRF protection
	state, err := generateRandomState()
	if err != nil {
		http.Error(w, "Failed to generate state", http.StatusInternalServerError)
		return
	}

	// Generate PKCE challenge for third-party OAuth if enabled
	var codeVerifier, codeChallenge string
	if c.config.UsePKCE {
		codeVerifier, codeChallenge, err = GenerateCodeVerifierAndChallenge()
		if err != nil {
			http.Error(w, "Failed to generate PKCE", http.StatusInternalServerError)
			return
		}
	}

	// Store state with MCP client parameters
	oauthState := &OAuthState{
		State:         state,
		CodeVerifier:  codeVerifier,
		CodeChallenge: codeChallenge,
		RedirectURI:   c.config.RedirectURI,
		CreatedAt:     time.Now(),
		ExpiresAt:     time.Now().Add(10 * time.Minute),

		// MCP Client parameters
		MCPClientID:            mcpClientID,
		MCPRedirectURI:         mcpRedirectURI,
		MCPScopes:              mcpScopes,
		MCPResources:           mcpResources,
		MCPState:               mcpState,
		MCPCodeChallenge:       mcpCodeChallenge,
		MCPCodeChallengeMethod: mcpCodeChallengeMethod,
	}

	c.stateMutex.Lock()
	c.stateStore[state] = oauthState
	c.stateMutex.Unlock()

	// Build authorization URL for third-party provider
	authURL := c.buildAuthorizationURL(state, codeChallenge)

	// Redirect user to third-party authorization page
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleCallback handles the callback from third-party OAuth provider
func (c *OAuthClient) HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Extract authorization code and state
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorCode := r.URL.Query().Get("error")

	// Check for errors from third-party
	if errorCode != "" {
		errorDesc := r.URL.Query().Get("error_description")
		http.Error(w, fmt.Sprintf("OAuth error: %s - %s", errorCode, errorDesc), http.StatusBadRequest)
		return
	}

	if code == "" || state == "" {
		http.Error(w, "Missing code or state parameter", http.StatusBadRequest)
		return
	}

	// Validate state
	c.stateMutex.RLock()
	oauthState, ok := c.stateStore[state]
	c.stateMutex.RUnlock()

	if !ok {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	// Check state expiration
	if time.Now().After(oauthState.ExpiresAt) {
		c.stateMutex.Lock()
		delete(c.stateStore, state)
		c.stateMutex.Unlock()
		http.Error(w, "State expired", http.StatusBadRequest)
		return
	}

	// Clean up used state
	c.stateMutex.Lock()
	delete(c.stateStore, state)
	c.stateMutex.Unlock()

	// Exchange authorization code for access token from third-party
	thirdPartyToken, err := c.exchangeCodeForToken(code, oauthState.CodeVerifier)
	if err != nil {
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	// Get user info from third-party
	userInfo, err := c.getUserInfo(thirdPartyToken.AccessToken)
	if err != nil {
		http.Error(w, "Failed to get user info", http.StatusInternalServerError)
		return
	}

	// Generate MCP authorization code with proper parameters
	mcpAuthCode, err := c.generateMCPAuthorizationCode(r.Context(), oauthState, userInfo, thirdPartyToken)
	if err != nil {
		http.Error(w, "Failed to complete authorization", http.StatusInternalServerError)
		return
	}

	// Redirect back to MCP client with MCP authorization code
	redirectURL := c.buildMCPClientRedirect(oauthState.MCPRedirectURI, mcpAuthCode, oauthState.MCPState)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// generateMCPAuthorizationCode generates a proper MCP authorization code
func (c *OAuthClient) generateMCPAuthorizationCode(
	ctx context.Context,
	oauthState *OAuthState,
	userInfo *ThirdPartyUserInfo,
	thirdPartyToken *ThirdPartyTokenResponse,
) (string, error) {
	code, err := GenerateAuthorizationCode()
	if err != nil {
		return "", err
	}

	// Set default PKCE method if challenge provided
	challengeMethod := oauthState.MCPCodeChallengeMethod
	if challengeMethod == "" && oauthState.MCPCodeChallenge != "" {
		challengeMethod = CodeChallengeMethodPlain
	}

	// Encrypt third-party tokens before storing
	encryptedAccessToken, err := c.encryptToken(thirdPartyToken.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt access token: %w", err)
	}

	encryptedRefreshToken := ""
	if thirdPartyToken.RefreshToken != "" {
		encryptedRefreshToken, err = c.encryptToken(thirdPartyToken.RefreshToken)
		if err != nil {
			return "", fmt.Errorf("failed to encrypt refresh token: %w", err)
		}
	}

	// Create authorization code with proper MCP client binding
	authCode := &AuthorizationCode{
		Code:                code,
		ClientID:            oauthState.MCPClientID,
		UserID:              userInfo.Subject,
		RedirectURI:         oauthState.MCPRedirectURI,
		Scopes:              oauthState.MCPScopes,
		Resources:           oauthState.MCPResources,
		CodeChallenge:       oauthState.MCPCodeChallenge,
		CodeChallengeMethod: challengeMethod,
		ExpiresAt:           time.Now().Add(5 * time.Minute),
		CreatedAt:           time.Now(),
		Used:                false,
		Metadata: map[string]interface{}{
			"third_party_provider":          c.config.ProviderName,
			"third_party_token_encrypted":   encryptedAccessToken,
			"third_party_refresh_encrypted": encryptedRefreshToken,
			"third_party_expires":           time.Now().Add(time.Duration(thirdPartyToken.ExpiresIn) * time.Second).Unix(),
			"user_email":                    userInfo.Email,
			"user_name":                     userInfo.Name,
		},
	}

	if err := c.store.SaveAuthorizationCode(ctx, authCode); err != nil {
		return "", err
	}

	return code, nil
}

// encryptToken encrypts sensitive tokens using AES-GCM
func (c *OAuthClient) encryptToken(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	c.encryptMutex.RLock()
	defer c.encryptMutex.RUnlock()

	block, err := aes.NewCipher(c.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptToken decrypts sensitive tokens
func (c *OAuthClient) decryptToken(encrypted string) (string, error) {
	if encrypted == "" {
		return "", nil
	}

	c.encryptMutex.RLock()
	defer c.encryptMutex.RUnlock()

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(c.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// exchangeCodeForToken exchanges authorization code for access token
func (c *OAuthClient) exchangeCodeForToken(code, codeVerifier string) (*ThirdPartyTokenResponse, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.config.RedirectURI)
	data.Set("client_id", c.config.ClientID)
	data.Set("client_secret", c.config.ClientSecret)

	if c.config.UsePKCE && codeVerifier != "" {
		data.Set("code_verifier", codeVerifier)
	}

	req, err := http.NewRequest(http.MethodPost, c.config.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token request failed: %d - %s", resp.StatusCode, string(body))
	}

	var tokenResp ThirdPartyTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// getUserInfo fetches user information from third-party
func (c *OAuthClient) getUserInfo(accessToken string) (*ThirdPartyUserInfo, error) {
	if c.config.UserInfoURL == "" {
		return &ThirdPartyUserInfo{
			Subject: "unknown",
			Claims:  make(map[string]interface{}),
		}, nil
	}

	req, err := http.NewRequest(http.MethodGet, c.config.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("userinfo request failed: %d - %s", resp.StatusCode, string(body))
	}

	var userInfo ThirdPartyUserInfo
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, err
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(body, &claims); err == nil {
		userInfo.Claims = claims
	}

	return &userInfo, nil
}

// buildAuthorizationURL builds the third-party authorization URL
func (c *OAuthClient) buildAuthorizationURL(state, codeChallenge string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", c.config.ClientID)
	params.Set("redirect_uri", c.config.RedirectURI)
	params.Set("state", state)
	params.Set("scope", strings.Join(c.config.Scopes, " "))

	if c.config.UsePKCE && codeChallenge != "" {
		params.Set("code_challenge", codeChallenge)
		params.Set("code_challenge_method", "S256")
	}

	return c.config.AuthorizationURL + "?" + params.Encode()
}

// buildMCPClientRedirect builds redirect URL back to MCP client
func (c *OAuthClient) buildMCPClientRedirect(mcpClientCallback, authCode, mcpState string) string {
	redirectURL, _ := url.Parse(mcpClientCallback)
	query := redirectURL.Query()
	query.Set("code", authCode)
	if mcpState != "" {
		query.Set("state", mcpState)
	}
	redirectURL.RawQuery = query.Encode()
	return redirectURL.String()
}

// generateRandomState generates a cryptographically secure random state
func generateRandomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
