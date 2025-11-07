package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// Server implements OAuth 2.1 authorization server
type Server struct {
	store            Store
	tokenGenerator   *TokenGenerator
	pkceValidator    *PKCEValidator
	config           *ServerConfig
	metadataProvider *MetadataProvider
	jwksProvider     *JWKSProvider
}

// ServerConfig holds OAuth server configuration
type ServerConfig struct {
	// Issuer identifier
	Issuer string

	// MCP Server URL (NEW - required by MCP spec)
	// The authorization base URL will be automatically derived from this
	// by discarding the path component
	MCPServerURL string

	// Supported grant types
	SupportedGrantTypes []GrantType

	// Authorization code TTL (short-lived, OAuth 2.1 recommends < 10 minutes)
	AuthorizationCodeTTL time.Duration

	// Require PKCE for all clients
	RequirePKCE bool

	// Require S256 challenge method
	RequireS256 bool

	// Enable refresh token rotation
	RefreshTokenRotation bool

	// Maximum number of redirect URIs per client
	MaxRedirectURIs int

	// RFC 8707: Resource indicators configuration
	SupportedResources []string

	// Logger for server operations
	Logger Logger

	// BaseURL is the derived authorization base URL
	// This is computed from MCPServerURL by discarding the path component
	baseURL string
}

// Logger interface
type Logger interface {
	Warnf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Errorf(format string, args ...interface{})
}

// DefaultLogger is a no-op logger
type DefaultLogger struct{}

func (d DefaultLogger) Warnf(format string, args ...interface{})  {}
func (d DefaultLogger) Infof(format string, args ...interface{})  {}
func (d DefaultLogger) Errorf(format string, args ...interface{}) {}

// DefaultServerConfig returns default OAuth 2.1 configuration
func DefaultServerConfig(issuer, mcpServerURL string) *ServerConfig {
	return &ServerConfig{
		Issuer:       issuer,
		MCPServerURL: mcpServerURL,
		SupportedGrantTypes: []GrantType{
			GrantTypeAuthorizationCode,
			GrantTypeClientCredentials,
			GrantTypeRefreshToken,
		},
		AuthorizationCodeTTL: 5 * time.Minute,
		RequirePKCE:          true, // OAuth 2.1 requirement
		RequireS256:          true, // OAuth 2.1 best practice
		RefreshTokenRotation: true, // OAuth 2.1 best practice
		MaxRedirectURIs:      10,
		SupportedResources:   DefaultMCPResources, // MCP resources
		Logger:               DefaultLogger{},
	}
}

// NewServer creates a new OAuth 2.1 server
func NewServer(store Store, signingKey []byte, config *ServerConfig) (*Server, error) {
	if store == nil {
		return nil, errors.New("store cannot be nil")
	}

	if len(signingKey) < 32 {
		return nil, errors.New("signing key must be at least 32 bytes")
	}

	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	// Validate MCP Server URL (required for base URL derivation)
	if config.MCPServerURL == "" {
		return nil, errors.New("MCPServerURL is required for MCP compliance")
	}

	// Validate MCP Server URL can be parsed
	if _, err := url.Parse(config.MCPServerURL); err != nil {
		return nil, fmt.Errorf("invalid MCPServerURL: %w", err)
	}

	// Set default logger if not provided
	if config.Logger == nil {
		config.Logger = DefaultLogger{}
	}

	tokenGenerator := NewTokenGenerator(signingKey, config.Issuer)

	pkceValidator := NewPKCEValidator()
	pkceValidator.RequirePKCE = config.RequirePKCE
	pkceValidator.RequireS256 = config.RequireS256

	// Create metadata provider - it will automatically derive authorization base URL
	metadataProvider, err := NewMetadataProvider(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create metadata provider: %w", err)
	}

	config.baseURL = metadataProvider.GetAuthorizationBaseURL()

	jwksProvider := NewJWKSProvider(signingKey, tokenGenerator.KeyID)

	config.Logger.Infof("OAuth server initialized with authorization base URL: %s (derived from MCP server URL: %s)",
		metadataProvider.GetAuthorizationBaseURL(), config.MCPServerURL)

	return &Server{
		store:            store,
		tokenGenerator:   tokenGenerator,
		pkceValidator:    pkceValidator,
		config:           config,
		metadataProvider: metadataProvider,
		jwksProvider:     jwksProvider,
	}, nil
}

// GetMetadataProvider returns the metadata provider
func (s *Server) GetMetadataProvider() *MetadataProvider {
	return s.metadataProvider
}

// GetJWKSProvider returns the JWKS provider
func (s *Server) GetJWKSProvider() *JWKSProvider {
	return s.jwksProvider
}

// GetAuthorizationBaseURL returns the automatically derived authorization base URL
// This is derived from MCPServerURL per MCP spec requirements
func (s *Server) GetAuthorizationBaseURL() string {
	return s.metadataProvider.GetAuthorizationBaseURL()
}

// GetBaseURL returns the base URL from server config
func (c *ServerConfig) GetBaseURL() string {
	return c.baseURL
}

// HandleAuthorizationRequest handles OAuth 2.1 authorization requests
func (s *Server) HandleAuthorizationRequest(ctx context.Context, req *AuthorizationRequest, userID string) (string, error) {
	// Validate response type (OAuth 2.1 only supports code)
	if req.ResponseType != "code" {
		return "", fmt.Errorf("%s: only 'code' response type is supported", ErrUnsupportedResponseType)
	}

	// Validate client
	client, err := s.store.GetClient(ctx, req.ClientID)
	if err != nil {
		return "", fmt.Errorf("%s: %w", ErrInvalidClient, err)
	}

	// Validate redirect URI
	if !s.validateRedirectURI(req.RedirectURI, client.RedirectURIs) {
		return "", fmt.Errorf("%s: invalid redirect_uri", ErrInvalidRequest)
	}

	// Parse and validate scopes
	requestedScopes := parseScopes(req.Scope)
	if !s.validateScopes(requestedScopes, client.Scopes) {
		return "", fmt.Errorf("%s: invalid scopes", ErrInvalidScope)
	}

	// RFC 8707: Validate resource indicators
	if len(req.Resource) > 0 {
		if err := s.validateResources(req.Resource, client.Resources); err != nil {
			return "", fmt.Errorf("%s: %w", ErrInvalidTarget, err)
		}
	}

	// Validate PKCE (required for public clients in OAuth 2.1)
	if err := s.pkceValidator.ValidateCodeChallenge(req.CodeChallenge, req.CodeChallengeMethod, client.IsPublic); err != nil {
		return "", fmt.Errorf("%s: %w", ErrInvalidRequest, err)
	}

	// Set default challenge method if not specified
	challengeMethod := req.CodeChallengeMethod
	if challengeMethod == "" && req.CodeChallenge != "" {
		challengeMethod = CodeChallengeMethodPlain
	}

	// Generate authorization code
	code, err := GenerateAuthorizationCode()
	if err != nil {
		return "", fmt.Errorf("%s: failed to generate code", ErrServerError)
	}

	// Store authorization code
	authCode := &AuthorizationCode{
		Code:                code,
		ClientID:            req.ClientID,
		UserID:              userID,
		RedirectURI:         req.RedirectURI,
		Scopes:              requestedScopes,
		Resources:           req.Resource, // RFC 8707
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: challengeMethod,
		ExpiresAt:           time.Now().Add(s.config.AuthorizationCodeTTL),
		CreatedAt:           time.Now(),
		Used:                false,
	}

	if err := s.store.SaveAuthorizationCode(ctx, authCode); err != nil {
		return "", fmt.Errorf("%s: failed to save authorization code", ErrServerError)
	}

	// Build redirect URI with code and state
	redirectURL, err := url.Parse(req.RedirectURI)
	if err != nil {
		return "", fmt.Errorf("%s: invalid redirect URI", ErrInvalidRequest)
	}

	query := redirectURL.Query()
	query.Set("code", code)
	if req.State != "" {
		query.Set("state", req.State)
	}
	redirectURL.RawQuery = query.Encode()

	return redirectURL.String(), nil
}

// HandleTokenRequest handles OAuth 2.1 token requests
func (s *Server) HandleTokenRequest(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	// Validate grant type
	grantType := GrantType(req.GrantType)
	if !s.isGrantTypeSupported(grantType) {
		return nil, fmt.Errorf("%s: %s", ErrUnsupportedGrantType, req.GrantType)
	}

	// Route to appropriate handler
	switch grantType {
	case GrantTypeAuthorizationCode:
		return s.handleAuthorizationCodeGrant(ctx, req)
	case GrantTypeClientCredentials:
		return s.handleClientCredentialsGrant(ctx, req)
	case GrantTypeRefreshToken:
		return s.handleRefreshTokenGrant(ctx, req)
	default:
		return nil, fmt.Errorf("%s: %s", ErrUnsupportedGrantType, req.GrantType)
	}
}

// handleAuthorizationCodeGrant handles authorization code flow
func (s *Server) handleAuthorizationCodeGrant(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	// Validate required parameters
	if req.Code == "" || req.RedirectURI == "" {
		return nil, fmt.Errorf("%s: missing required parameters", ErrInvalidRequest)
	}

	// Authenticate client
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Retrieve authorization code
	authCode, err := s.store.GetAuthorizationCode(ctx, req.Code)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%s: invalid authorization code", ErrInvalidGrant)
		}
		return nil, fmt.Errorf("%s: %w", ErrServerError, err)
	}

	// Validate authorization code
	if authCode.Used {
		return nil, fmt.Errorf("%s: authorization code already used", ErrInvalidGrant)
	}

	if time.Now().After(authCode.ExpiresAt) {
		return nil, fmt.Errorf("%s: authorization code expired", ErrInvalidGrant)
	}

	if authCode.ClientID != client.ID {
		return nil, fmt.Errorf("%s: client mismatch", ErrInvalidGrant)
	}

	if authCode.RedirectURI != req.RedirectURI {
		return nil, fmt.Errorf("%s: redirect_uri mismatch", ErrInvalidGrant)
	}

	// Verify PKCE if code challenge was used
	if authCode.CodeChallenge != "" {
		if req.CodeVerifier == "" {
			return nil, fmt.Errorf("%s: code_verifier required", ErrInvalidRequest)
		}

		if err := VerifyCodeChallenge(req.CodeVerifier, authCode.CodeChallenge, authCode.CodeChallengeMethod); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrInvalidGrant, err)
		}
	}

	// Invalidate authorization code (one-time use)
	if err := s.store.InvalidateAuthorizationCode(ctx, req.Code); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrServerError, err)
	}

	// Use resources from authorization code
	resources := authCode.Resources

	// Generate tokens
	return s.generateTokenPair(ctx, client.ID, authCode.UserID, authCode.Scopes, resources, true)
}

// handleClientCredentialsGrant handles client credentials flow
func (s *Server) handleClientCredentialsGrant(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	// Authenticate client
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Parse requested scopes
	requestedScopes := parseScopes(req.Scope)
	if !s.validateScopes(requestedScopes, client.Scopes) {
		return nil, fmt.Errorf("%s: invalid scopes", ErrInvalidScope)
	}

	// RFC 8707: Validate resource indicators
	resources := req.Resource
	if len(resources) > 0 {
		if err := s.validateResources(resources, client.Resources); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrInvalidTarget, err)
		}
	}

	// Generate access token only (no refresh token for client credentials)
	return s.generateTokenPair(ctx, client.ID, "", requestedScopes, resources, false)
}

// handleRefreshTokenGrant handles refresh token flow
func (s *Server) handleRefreshTokenGrant(ctx context.Context, req *TokenRequest) (*TokenResponse, error) {
	if req.RefreshToken == "" {
		return nil, fmt.Errorf("%s: missing refresh_token", ErrInvalidRequest)
	}

	// Authenticate client
	client, err := s.authenticateClient(ctx, req.ClientID, req.ClientSecret)
	if err != nil {
		return nil, err
	}

	// Retrieve refresh token
	refreshToken, err := s.store.GetRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%s: invalid refresh token", ErrInvalidGrant)
		}
		return nil, fmt.Errorf("%s: %w", ErrServerError, err)
	}

	// Validate refresh token
	if refreshToken.Revoked {
		return nil, fmt.Errorf("%s: refresh token revoked", ErrInvalidGrant)
	}

	if time.Now().After(refreshToken.ExpiresAt) {
		return nil, fmt.Errorf("%s: refresh token expired", ErrInvalidGrant)
	}

	if refreshToken.ClientID != client.ID {
		return nil, fmt.Errorf("%s: client mismatch", ErrInvalidGrant)
	}

	// Handle token rotation (OAuth 2.1 best practice)
	if s.config.RefreshTokenRotation {
		// Generate new refresh token
		newRefreshToken, err := s.tokenGenerator.GenerateRefreshToken(
			client.ID,
			refreshToken.UserID,
			refreshToken.Scopes,
			refreshToken.Resources,
		)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to generate refresh token", ErrServerError)
		}

		newRefreshToken.RotationCount = refreshToken.RotationCount + 1

		// Rotate tokens atomically
		if err := s.store.RotateRefreshToken(ctx, req.RefreshToken, newRefreshToken); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrServerError, err)
		}

		refreshToken = newRefreshToken
	}

	// Generate new access token with same resources
	accessToken, err := s.tokenGenerator.GenerateAccessToken(
		client.ID,
		refreshToken.UserID,
		refreshToken.Scopes,
		refreshToken.Resources,
	)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to generate access token", ErrServerError)
	}

	// Store access token
	if err := s.store.SaveAccessToken(ctx, accessToken); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrServerError, err)
	}

	return CreateTokenResponse(accessToken, refreshToken), nil
}

// generateTokenPair generates access and optionally refresh token
func (s *Server) generateTokenPair(
	ctx context.Context,
	clientID, userID string,
	scopes, resources []string,
	includeRefreshToken bool,
) (*TokenResponse, error) {
	accessToken, err := s.tokenGenerator.GenerateAccessToken(clientID, userID, scopes, resources)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to generate access token", ErrServerError)
	}

	// Store access token
	if err := s.store.SaveAccessToken(ctx, accessToken); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrServerError, err)
	}

	var refreshToken *RefreshToken
	if includeRefreshToken {
		refreshToken, err = s.tokenGenerator.GenerateRefreshToken(clientID, userID, scopes, resources)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to generate refresh token", ErrServerError)
		}

		// Store refresh token
		if err := s.store.SaveRefreshToken(ctx, refreshToken); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrServerError, err)
		}
	}

	return CreateTokenResponse(accessToken, refreshToken), nil
}

// authenticateClient authenticates the client
func (s *Server) authenticateClient(ctx context.Context, clientID, clientSecret string) (*Client, error) {
	if clientID == "" {
		return nil, fmt.Errorf("%s: missing client_id", ErrInvalidClient)
	}

	client, err := s.store.GetClient(ctx, clientID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, fmt.Errorf("%s: client not found", ErrInvalidClient)
		}
		return nil, fmt.Errorf("%s: %w", ErrServerError, err)
	}

	// Public clients don't require secret
	if client.IsPublic {
		return client, nil
	}

	// Validate client secret for confidential clients
	if err := s.store.ValidateClientSecret(ctx, clientID, clientSecret); err != nil {
		return nil, fmt.Errorf("%s: invalid credentials", ErrInvalidClient)
	}

	return client, nil
}

// Helper functions

func (s *Server) validateRedirectURI(redirectURI string, allowedURIs []string) bool {
	for _, allowed := range allowedURIs {
		if redirectURI == allowed {
			return true
		}
	}
	return false
}

func (s *Server) validateScopes(requested, allowed []string) bool {
	allowedMap := make(map[string]bool)
	for _, scope := range allowed {
		allowedMap[scope] = true
	}

	for _, scope := range requested {
		if !allowedMap[scope] {
			return false
		}
	}

	return true
}

// validateResources validates resource indicators (RFC 8707)
func (s *Server) validateResources(requested, allowed []string) error {
	if len(allowed) == 0 {
		// If client has no resource restrictions, check against server supported resources
		allowed = s.config.SupportedResources
	}

	if len(allowed) == 0 {
		// No restrictions
		return nil
	}

	allowedMap := make(map[string]bool)
	for _, res := range allowed {
		allowedMap[res] = true
	}

	for _, res := range requested {
		if !allowedMap[res] {
			return fmt.Errorf("resource not allowed: %s", res)
		}
	}

	return nil
}

func (s *Server) isGrantTypeSupported(grantType GrantType) bool {
	for _, supported := range s.config.SupportedGrantTypes {
		if grantType == supported {
			return true
		}
	}
	return false
}
