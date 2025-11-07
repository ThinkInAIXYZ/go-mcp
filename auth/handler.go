package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Handler provides HTTP handlers for OAuth 2.1 endpoints
type Handler struct {
	server                     *Server
	oauthClient                *OAuthClient
	dynamicRegistrationHandler *DynamicRegistrationHandler
}

// NewHandler creates a new OAuth handler
func NewHandler(server *Server) *Handler {
	return &Handler{
		server:                     server,
		dynamicRegistrationHandler: NewDynamicRegistrationHandler(server, nil),
	}
}

func (h *Handler) SetDynamicRegistrationConfig(config *DynamicRegistrationConfig) {
	h.dynamicRegistrationHandler = NewDynamicRegistrationHandler(h.server, config)
}

// HandleAuthorization handles GET /oauth/authorize
func (h *Handler) HandleAuthorization(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	// MCP spec: Check MCP-Protocol-Version header (optional but recommended)
	mcpVersion := r.Header.Get(MCPProtocolVersionHeader)
	if mcpVersion != "" && mcpVersion != CurrentMCPVersion {
		h.server.config.Logger.Warnf("Client using MCP version %s, server supports %s", mcpVersion, CurrentMCPVersion)
		// Continue - version mismatch is a warning, not an error
	}

	// Parse authorization request
	req := &AuthorizationRequest{
		ResponseType:        r.URL.Query().Get("response_type"),
		ClientID:            r.URL.Query().Get("client_id"),
		RedirectURI:         r.URL.Query().Get("redirect_uri"),
		Scope:               r.URL.Query().Get("scope"),
		State:               r.URL.Query().Get("state"),
		CodeChallenge:       r.URL.Query().Get("code_challenge"),
		CodeChallengeMethod: r.URL.Query().Get("code_challenge_method"),
		Resource:            r.URL.Query()["resource"], // RFC 8707
	}

	// State parameter is required for CSRF protection
	if err := validateState(req.State); err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}

	// Validate client ID
	if req.ClientID == "" {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, "client_id is required")
		return
	}

	// Get client
	client, err := h.server.store.GetClient(r.Context(), req.ClientID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidClient, "invalid client_id")
		return
	}

	// Validate redirect URI with exact matching
	if err := validateRedirectURI(client.RedirectURIs, req.RedirectURI); err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, err.Error())
		return
	}

	// MCP spec: Enforce HTTPS or localhost for redirect URIs
	if err := validateEndpointSecurity(req.RedirectURI); err != nil {
		h.redirectWithError(w, r, req.RedirectURI, req.State, err)
		return
	}

	// Validate response type
	if req.ResponseType != "code" {
		h.redirectWithError(w, r, req.RedirectURI, req.State,
			fmt.Errorf("%s: only 'code' response_type is supported", ErrUnsupportedResponseType))
		return
	}

	// PKCE is required for public clients
	if client.IsPublic && req.CodeChallenge == "" {
		h.redirectWithError(w, r, req.RedirectURI, req.State,
			fmt.Errorf("%s: PKCE is required for public clients", ErrInvalidRequest))
		return
	}

	// Validate PKCE if provided
	if req.CodeChallenge != "" {
		pkceValidator := NewPKCEValidator()
		if err := pkceValidator.ValidateCodeChallenge(req.CodeChallenge, req.CodeChallengeMethod, client.IsPublic); err != nil {
			h.redirectWithError(w, r, req.RedirectURI, req.State, err)
			return
		}
	}

	// Validate scopes
	requestedScopes := parseScopes(req.Scope)
	if err := validateScope(requestedScopes, client.Scopes); err != nil {
		h.redirectWithError(w, r, req.RedirectURI, req.State, err)
		return
	}

	// Get user ID (in production, this would come from user authentication)
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		h.redirectWithError(w, r, req.RedirectURI, req.State,
			fmt.Errorf("%s: user not authenticated", ErrAccessDenied))
		return
	}

	// Handle authorization request
	redirectURL, err := h.server.HandleAuthorizationRequest(r.Context(), req, userID)
	if err != nil {
		h.redirectWithError(w, r, req.RedirectURI, req.State, err)
		return
	}

	// Redirect to client with authorization code
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// HandleToken handles POST /oauth/token
func (h *Handler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	// MCP spec: Check MCP-Protocol-Version header
	mcpVersion := r.Header.Get(MCPProtocolVersionHeader)
	if mcpVersion != "" && mcpVersion != CurrentMCPVersion {
		h.server.config.Logger.Warnf("Client using MCP version %s for token request", mcpVersion)
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, "invalid form data")
		return
	}

	// Extract token request
	req := &TokenRequest{
		GrantType:    r.FormValue("grant_type"),
		Code:         r.FormValue("code"),
		RedirectURI:  r.FormValue("redirect_uri"),
		ClientID:     r.FormValue("client_id"),
		ClientSecret: r.FormValue("client_secret"),
		RefreshToken: r.FormValue("refresh_token"),
		Scope:        r.FormValue("scope"),
		Resource:     r.Form["resource"], // RFC 8707
		CodeVerifier: r.FormValue("code_verifier"),
	}

	// Try Basic Auth for client credentials
	if req.ClientID == "" || req.ClientSecret == "" {
		clientID, clientSecret, ok := r.BasicAuth()
		if ok {
			req.ClientID = clientID
			req.ClientSecret = clientSecret
		}
	}

	// Handle token request
	tokenResp, err := h.server.HandleTokenRequest(r.Context(), req)
	if err != nil {
		h.writeErrorFromErr(w, err)
		return
	}

	// Write successful response
	h.writeJSON(w, http.StatusOK, tokenResp)
}

// HandleIntrospection handles POST /oauth/introspect
func (h *Handler) HandleIntrospection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, "invalid form data")
		return
	}

	token := r.FormValue("token")
	if token == "" {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, "missing token parameter")
		return
	}

	// Authenticate client (introspection requires authentication)
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	if clientID == "" {
		h.writeError(w, http.StatusUnauthorized, ErrInvalidClient, "client authentication required")
		return
	}

	// Validate client
	if err := h.server.store.ValidateClientSecret(r.Context(), clientID, clientSecret); err != nil {
		h.writeError(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
		return
	}

	// Introspect token
	resp := h.introspectToken(r.Context(), token)
	h.writeJSON(w, http.StatusOK, resp)
}

// HandleRevocation handles POST /oauth/revoke
func (h *Handler) HandleRevocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "method not allowed")
		return
	}

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, "invalid form data")
		return
	}

	token := r.FormValue("token")
	tokenTypeHint := r.FormValue("token_type_hint")

	if token == "" {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, "missing token parameter")
		return
	}

	// Authenticate client
	clientID, clientSecret, ok := r.BasicAuth()
	if !ok {
		clientID = r.FormValue("client_id")
		clientSecret = r.FormValue("client_secret")
	}

	if clientID == "" {
		h.writeError(w, http.StatusUnauthorized, ErrInvalidClient, "client authentication required")
		return
	}

	// Validate client
	if err := h.server.store.ValidateClientSecret(r.Context(), clientID, clientSecret); err != nil {
		h.writeError(w, http.StatusUnauthorized, ErrInvalidClient, "invalid client credentials")
		return
	}

	// Revoke token
	ctx := r.Context()
	if tokenTypeHint == "refresh_token" {
		_ = h.server.store.RevokeRefreshToken(ctx, token)
	} else {
		_ = h.server.store.RevokeAccessToken(ctx, token)
		_ = h.server.store.RevokeRefreshToken(ctx, token)
	}

	// RFC 7009: The authorization server responds with HTTP status code 200
	w.WriteHeader(http.StatusOK)
}

// introspectToken introspects an access token
func (h *Handler) introspectToken(ctx context.Context, token string) *IntrospectionResponse {
	// Try to validate JWT token
	claims, err := h.server.tokenGenerator.ValidateAccessToken(token)
	if err != nil {
		return &IntrospectionResponse{Active: false}
	}

	// Verify token exists in store
	accessToken, err := h.server.store.GetAccessToken(ctx, token)
	if err != nil {
		return &IntrospectionResponse{Active: false}
	}

	// Build introspection response with audience (RFC 8707)
	return &IntrospectionResponse{
		Active:    true,
		Scope:     strings.Join(accessToken.Scopes, " "),
		ClientID:  accessToken.ClientID,
		Username:  accessToken.UserID,
		TokenType: string(accessToken.TokenType),
		ExpiresAt: accessToken.ExpiresAt.Unix(),
		IssuedAt:  accessToken.CreatedAt.Unix(),
		Subject:   claims.Subject,
		Audience:  accessToken.Resources, // RFC 8707
	}
}

// Helper methods for HTTP responses

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, errorCode, description string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)

	errorResp := &ErrorResponse{
		Error:            errorCode,
		ErrorDescription: description,
	}

	json.NewEncoder(w).Encode(errorResp)
}

func (h *Handler) writeErrorFromErr(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	errorCode := ErrInvalidRequest
	description := err.Error()

	// Parse error to determine status code
	errStr := err.Error()
	if strings.Contains(errStr, ErrInvalidClient) {
		status = http.StatusUnauthorized
		errorCode = ErrInvalidClient
	} else if strings.Contains(errStr, ErrInvalidGrant) {
		errorCode = ErrInvalidGrant
	} else if strings.Contains(errStr, ErrUnauthorizedClient) {
		errorCode = ErrUnauthorizedClient
	} else if strings.Contains(errStr, ErrUnsupportedGrantType) {
		errorCode = ErrUnsupportedGrantType
	} else if strings.Contains(errStr, ErrInvalidScope) {
		errorCode = ErrInvalidScope
	} else if strings.Contains(errStr, ErrInvalidTarget) {
		errorCode = ErrInvalidTarget
	} else if strings.Contains(errStr, ErrServerError) {
		status = http.StatusInternalServerError
		errorCode = ErrServerError
	}

	h.writeError(w, status, errorCode, description)
}

func (h *Handler) redirectWithError(w http.ResponseWriter, r *http.Request, redirectURI, state string, err error) {
	if redirectURI == "" {
		h.writeErrorFromErr(w, err)
		return
	}

	errorCode := ErrServerError
	description := err.Error()

	// Determine error code
	errStr := err.Error()
	if strings.Contains(errStr, ErrInvalidRequest) {
		errorCode = ErrInvalidRequest
	} else if strings.Contains(errStr, ErrUnauthorizedClient) {
		errorCode = ErrUnauthorizedClient
	} else if strings.Contains(errStr, ErrAccessDenied) {
		errorCode = ErrAccessDenied
	} else if strings.Contains(errStr, ErrUnsupportedResponseType) {
		errorCode = ErrUnsupportedResponseType
	} else if strings.Contains(errStr, ErrInvalidScope) {
		errorCode = ErrInvalidScope
	} else if strings.Contains(errStr, ErrInvalidTarget) {
		errorCode = ErrInvalidTarget
	}

	// Build error redirect URL
	redirectURL, _ := url.Parse(redirectURI)
	query := redirectURL.Query()
	query.Set("error", errorCode)
	query.Set("error_description", description)
	if state != "" {
		query.Set("state", state)
	}
	redirectURL.RawQuery = query.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// RegisterRoutes registers OAuth endpoints on a mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/oauth/authorize", h.HandleAuthorization)
	mux.HandleFunc("/oauth/token", h.HandleToken)
	mux.HandleFunc("/oauth/introspect", h.HandleIntrospection)
	mux.HandleFunc("/oauth/revoke", h.HandleRevocation)

	// RFC 7591: Dynamic Client Registration
	mux.HandleFunc("/register", h.dynamicRegistrationHandler.HandleRegister)
	mux.HandleFunc("/register/", h.dynamicRegistrationHandler.HandleClientConfiguration)

	// RFC 8414: Authorization Server Metadata
	mux.Handle("/.well-known/oauth-authorization-server", h.server.GetMetadataProvider())

	// RFC 9728: Protected Resource Metadata (optional)
	mux.HandleFunc("/.well-known/oauth-protected-resource", h.server.GetMetadataProvider().ServeProtectedResourceMetadata)

	// JWKS endpoint
	mux.Handle("/.well-known/jwks.json", h.server.GetJWKSProvider())
}

// RegisterRoutesWithPrefix registers OAuth endpoints with a custom prefix
func (h *Handler) RegisterRoutesWithPrefix(mux *http.ServeMux, prefix string) {
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if strings.HasSuffix(prefix, "/") {
		prefix = strings.TrimSuffix(prefix, "/")
	}

	mux.HandleFunc(prefix+"/authorize", h.HandleAuthorization)
	mux.HandleFunc(prefix+"/token", h.HandleToken)
	mux.HandleFunc(prefix+"/introspect", h.HandleIntrospection)
	mux.HandleFunc(prefix+"/revoke", h.HandleRevocation)

	// RFC 7591: Dynamic Client Registration
	mux.HandleFunc("/register", h.dynamicRegistrationHandler.HandleRegister)
	mux.HandleFunc("/register/", h.dynamicRegistrationHandler.HandleClientConfiguration)

	// Metadata and JWKS at well-known locations (not prefixed per spec)
	mux.Handle("/.well-known/oauth-authorization-server", h.server.GetMetadataProvider())
	mux.HandleFunc("/.well-known/oauth-protected-resource", h.server.GetMetadataProvider().ServeProtectedResourceMetadata)
	mux.Handle("/.well-known/jwks.json", h.server.GetJWKSProvider())
}

func (h *Handler) SetOAuthClient(client *OAuthClient) {
	h.oauthClient = client
}

func (h *Handler) RegisterRoutesWithOAuth(mux *http.ServeMux) {
	// Standard OAuth Endpoints
	mux.HandleFunc("/oauth/authorize", h.HandleAuthorization)
	mux.HandleFunc("/oauth/token", h.HandleToken)
	mux.HandleFunc("/oauth/introspect", h.HandleIntrospection)
	mux.HandleFunc("/oauth/revoke", h.HandleRevocation)

	// RFC 7591: Dynamic Client Registration
	mux.HandleFunc("/register", h.dynamicRegistrationHandler.HandleRegister)
	mux.HandleFunc("/register/", h.dynamicRegistrationHandler.HandleClientConfiguration)

	// Metadata and JWKS
	mux.Handle("/.well-known/oauth-authorization-server", h.server.GetMetadataProvider())
	mux.HandleFunc("/.well-known/oauth-protected-resource", h.server.GetMetadataProvider().ServeProtectedResourceMetadata)
	mux.Handle("/.well-known/jwks.json", h.server.GetJWKSProvider())

	// Third-party OAuth flow endpoints
	if h.oauthClient != nil {
		mux.HandleFunc("/oauth/login", h.oauthClient.InitiateOAuthFlow)
		mux.HandleFunc("/oauth/callback", h.oauthClient.HandleCallback)
	}
}

func (h *Handler) GetDynamicRegistrationHandler() *DynamicRegistrationHandler {
	return h.dynamicRegistrationHandler
}
