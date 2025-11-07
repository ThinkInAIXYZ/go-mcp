package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RFC 7591: OAuth 2.0 Dynamic Client Registration Protocol

// ClientRegistrationRequest represents a dynamic client registration request
type ClientRegistrationRequest struct {
	// OPTIONAL: Array of redirection URIs
	RedirectURIs []string `json:"redirect_uris,omitempty"`

	// OPTIONAL: Array of OAuth 2.0 grant types
	GrantTypes []string `json:"grant_types,omitempty"`

	// OPTIONAL: Kind of application (e.g., "web", "native")
	ApplicationType string `json:"application_type,omitempty"`

	// OPTIONAL: Array of email addresses
	Contacts []string `json:"contacts,omitempty"`

	// OPTIONAL: Human-readable client name
	ClientName string `json:"client_name,omitempty"`

	// OPTIONAL: URL of the client logo
	LogoURI string `json:"logo_uri,omitempty"`

	// OPTIONAL: URL of the client's home page
	ClientURI string `json:"client_uri,omitempty"`

	// OPTIONAL: URL of policy document
	PolicyURI string `json:"policy_uri,omitempty"`

	// OPTIONAL: URL of terms of service
	TosURI string `json:"tos_uri,omitempty"`

	// OPTIONAL: URL for the client's JSON Web Key Set
	JWKSURI string `json:"jwks_uri,omitempty"`

	// OPTIONAL: Client's JSON Web Key Set
	JWKS json.RawMessage `json:"jwks,omitempty"`

	// OPTIONAL: URL referencing the software statement
	SoftwareID string `json:"software_id,omitempty"`

	// OPTIONAL: Version identifier for the software
	SoftwareVersion string `json:"software_version,omitempty"`

	// OPTIONAL: Requested authentication method for the token endpoint
	TokenEndpointAuthMethod string `json:"token_endpoint_auth_method,omitempty"`

	// OPTIONAL: Requested scope values
	Scope string `json:"scope,omitempty"`

	// RFC 8707: Resource indicators
	ResourceIndicators []string `json:"resource_indicators,omitempty"`

	// MCP-specific extensions
	MCPVersion             string   `json:"mcp_version,omitempty"`
	MCPTransportsSupported []string `json:"mcp_transports_supported,omitempty"`
}

// ClientRegistrationResponse represents a successful registration response
type ClientRegistrationResponse struct {
	// REQUIRED: Unique client identifier
	ClientID string `json:"client_id"`

	// OPTIONAL: Client secret (only for confidential clients)
	ClientSecret string `json:"client_secret,omitempty"`

	// OPTIONAL: Time at which the client secret will expire (Unix timestamp)
	ClientSecretExpiresAt int64 `json:"client_secret_expires_at,omitempty"`

	// OPTIONAL: Registration access token for managing this client
	RegistrationAccessToken string `json:"registration_access_token,omitempty"`

	// OPTIONAL: URL for managing this client registration
	RegistrationClientURI string `json:"registration_client_uri,omitempty"`

	// Echo back the registered parameters
	RedirectURIs            []string        `json:"redirect_uris,omitempty"`
	GrantTypes              []string        `json:"grant_types,omitempty"`
	ApplicationType         string          `json:"application_type,omitempty"`
	Contacts                []string        `json:"contacts,omitempty"`
	ClientName              string          `json:"client_name,omitempty"`
	LogoURI                 string          `json:"logo_uri,omitempty"`
	ClientURI               string          `json:"client_uri,omitempty"`
	PolicyURI               string          `json:"policy_uri,omitempty"`
	TosURI                  string          `json:"tos_uri,omitempty"`
	JWKSURI                 string          `json:"jwks_uri,omitempty"`
	JWKS                    json.RawMessage `json:"jwks,omitempty"`
	SoftwareID              string          `json:"software_id,omitempty"`
	SoftwareVersion         string          `json:"software_version,omitempty"`
	TokenEndpointAuthMethod string          `json:"token_endpoint_auth_method,omitempty"`
	Scope                   string          `json:"scope,omitempty"`

	// RFC 8707: Resource indicators
	ResourceIndicators []string `json:"resource_indicators,omitempty"`

	// MCP-specific
	MCPVersion             string   `json:"mcp_version,omitempty"`
	MCPTransportsSupported []string `json:"mcp_transports_supported,omitempty"`

	// Metadata
	ClientIDIssuedAt int64 `json:"client_id_issued_at,omitempty"`
}

// ClientUpdateRequest represents a client update request
type ClientUpdateRequest struct {
	ClientRegistrationRequest
	ClientID string `json:"client_id"`
}

// DynamicRegistrationConfig configures dynamic client registration behavior
type DynamicRegistrationConfig struct {
	// Enable or disable dynamic registration
	Enabled bool

	// Require initial access token for registration
	RequireInitialAccessToken bool

	// Initial access token (if required)
	InitialAccessToken string

	// Allow public clients (no client_secret)
	AllowPublicClients bool

	// Default grant types if not specified
	DefaultGrantTypes []GrantType

	// Default token endpoint auth method
	DefaultTokenEndpointAuthMethod string

	// Maximum number of redirect URIs
	MaxRedirectURIs int

	// Client secret expiration time (0 = never expires)
	ClientSecretTTL time.Duration

	// Generate registration access token for client management
	EnableRegistrationAccessToken bool

	// Allowed application types
	AllowedApplicationTypes []string

	// Validate redirect URIs
	RedirectURIValidator func(string, string) error

	// Custom client ID generator
	ClientIDGenerator func() (string, error)

	// Custom client secret generator
	ClientSecretGenerator func() (string, error)
}

// DefaultDynamicRegistrationConfig returns default configuration
func DefaultDynamicRegistrationConfig() *DynamicRegistrationConfig {
	return &DynamicRegistrationConfig{
		Enabled:                        true,
		RequireInitialAccessToken:      false,
		AllowPublicClients:             true,
		DefaultGrantTypes:              []GrantType{GrantTypeAuthorizationCode, GrantTypeRefreshToken},
		DefaultTokenEndpointAuthMethod: "client_secret_basic",
		MaxRedirectURIs:                10,
		ClientSecretTTL:                0, // Never expires
		EnableRegistrationAccessToken:  true,
		AllowedApplicationTypes:        []string{"web", "native"},
		RedirectURIValidator:           DefaultRedirectURIValidator,
		ClientIDGenerator:              GenerateClientID,
		ClientSecretGenerator:          GenerateClientSecret,
	}
}

// DynamicRegistrationHandler handles dynamic client registration
type DynamicRegistrationHandler struct {
	server *Server
	config *DynamicRegistrationConfig
	store  Store
}

// NewDynamicRegistrationHandler creates a new registration handler
func NewDynamicRegistrationHandler(server *Server, config *DynamicRegistrationConfig) *DynamicRegistrationHandler {
	if config == nil {
		config = DefaultDynamicRegistrationConfig()
	}

	return &DynamicRegistrationHandler{
		server: server,
		config: config,
		store:  server.store,
	}
}

// HandleRegister handles POST /register
func (h *DynamicRegistrationHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if !h.config.Enabled {
		h.writeError(w, http.StatusNotFound, ErrInvalidRequest, "Dynamic registration is not enabled")
		return
	}

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "Method not allowed")
		return
	}

	// Validate initial access token if required
	if h.config.RequireInitialAccessToken {
		token := extractBearerToken(r)
		if token != h.config.InitialAccessToken {
			h.writeError(w, http.StatusUnauthorized, ErrInvalidToken, "Invalid or missing initial access token")
			return
		}
	}

	// Parse registration request
	var req ClientRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	// Register the client
	resp, err := h.registerClient(r.Context(), &req)
	if err != nil {
		h.writeErrorFromErr(w, err)
		return
	}

	// Write response
	h.writeJSON(w, http.StatusCreated, resp)
}

// HandleClientConfiguration handles GET/PUT/DELETE /register/:client_id
func (h *DynamicRegistrationHandler) HandleClientConfiguration(w http.ResponseWriter, r *http.Request) {
	if !h.config.Enabled {
		h.writeError(w, http.StatusNotFound, ErrInvalidRequest, "Dynamic registration is not enabled")
		return
	}

	// Extract client ID from path
	clientID := extractClientIDFromPath(r.URL.Path)
	if clientID == "" {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, "Missing client ID")
		return
	}

	// Validate registration access token
	token := extractBearerToken(r)
	if token == "" {
		h.writeError(w, http.StatusUnauthorized, ErrInvalidToken, "Missing registration access token")
		return
	}

	// Verify token matches client
	if err := h.validateRegistrationAccessToken(r.Context(), clientID, token); err != nil {
		h.writeError(w, http.StatusUnauthorized, ErrInvalidToken, "Invalid registration access token")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetClient(w, r, clientID)
	case http.MethodPut:
		h.handleUpdateClient(w, r, clientID)
	case http.MethodDelete:
		h.handleDeleteClient(w, r, clientID)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, ErrInvalidRequest, "Method not allowed")
	}
}

// registerClient registers a new client
func (h *DynamicRegistrationHandler) registerClient(ctx context.Context, req *ClientRegistrationRequest) (*ClientRegistrationResponse, error) {
	// Validate redirect URIs
	if len(req.RedirectURIs) == 0 {
		return nil, fmt.Errorf("%s: at least one redirect_uri is required", ErrInvalidRequest)
	}

	if len(req.RedirectURIs) > h.config.MaxRedirectURIs {
		return nil, fmt.Errorf("%s: too many redirect URIs (max: %d)", ErrInvalidRequest, h.config.MaxRedirectURIs)
	}

	// Validate application type
	applicationType := req.ApplicationType
	if applicationType == "" {
		applicationType = "web"
	}

	if !contains(h.config.AllowedApplicationTypes, applicationType) {
		return nil, fmt.Errorf("%s: unsupported application type: %s", ErrInvalidRequest, applicationType)
	}

	// Validate redirect URIs based on application type
	for _, uri := range req.RedirectURIs {
		if err := h.config.RedirectURIValidator(uri, applicationType); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrInvalidRequest, err)
		}
	}

	// Determine grant types
	grantTypes := req.GrantTypes
	if len(grantTypes) == 0 {
		grantTypes = make([]string, len(h.config.DefaultGrantTypes))
		for i, gt := range h.config.DefaultGrantTypes {
			grantTypes[i] = string(gt)
		}
	}

	// Validate grant types
	for _, gt := range grantTypes {
		if !h.server.isGrantTypeSupported(GrantType(gt)) {
			return nil, fmt.Errorf("%s: unsupported grant type: %s", ErrInvalidRequest, gt)
		}
	}

	// Determine if client is public or confidential
	isPublic := applicationType == "native" || req.TokenEndpointAuthMethod == "none"

	if isPublic && !h.config.AllowPublicClients {
		return nil, fmt.Errorf("%s: public clients are not allowed", ErrInvalidRequest)
	}

	// Generate client ID
	clientID, err := h.config.ClientIDGenerator()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to generate client ID", ErrServerError)
	}

	// Generate client secret for confidential clients
	var clientSecret string
	var clientSecretExpiresAt int64
	if !isPublic {
		clientSecret, err = h.config.ClientSecretGenerator()
		if err != nil {
			return nil, fmt.Errorf("%s: failed to generate client secret", ErrServerError)
		}

		if h.config.ClientSecretTTL > 0 {
			clientSecretExpiresAt = time.Now().Add(h.config.ClientSecretTTL).Unix()
		}
	}

	// Parse scopes
	scopes := parseScopes(req.Scope)
	if len(scopes) == 0 {
		// Default scopes for MCP
		scopes = []string{"read", "write"}
	}

	// Parse resources (RFC 8707)
	resources := req.ResourceIndicators
	if len(resources) == 0 && len(h.server.config.SupportedResources) > 0 {
		resources = h.server.config.SupportedResources
	}

	// Create client
	now := time.Now()
	client := &Client{
		ID:           clientID,
		Secret:       clientSecret,
		Name:         req.ClientName,
		RedirectURIs: req.RedirectURIs,
		GrantTypes:   grantTypes,
		Scopes:       scopes,
		Resources:    resources,
		IsPublic:     isPublic,
		CreatedAt:    now,
		UpdatedAt:    now,
		Metadata: map[string]interface{}{
			"application_type":           applicationType,
			"contacts":                   req.Contacts,
			"logo_uri":                   req.LogoURI,
			"client_uri":                 req.ClientURI,
			"policy_uri":                 req.PolicyURI,
			"tos_uri":                    req.TosURI,
			"jwks_uri":                   req.JWKSURI,
			"software_id":                req.SoftwareID,
			"software_version":           req.SoftwareVersion,
			"token_endpoint_auth_method": req.TokenEndpointAuthMethod,
			"mcp_version":                req.MCPVersion,
			"mcp_transports_supported":   req.MCPTransportsSupported,
			"client_secret_expires_at":   clientSecretExpiresAt,
		},
	}

	// Store client
	if err := h.store.CreateClient(ctx, client); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrServerError, err)
	}

	// Generate registration access token if enabled
	var registrationAccessToken string
	var registrationClientURI string
	if h.config.EnableRegistrationAccessToken {
		registrationAccessToken, err = GenerateRegistrationAccessToken()
		if err != nil {
			return nil, fmt.Errorf("%s: failed to generate registration access token", ErrServerError)
		}

		// Store the token (in metadata for now, in production use a separate store)
		client.Metadata["registration_access_token"] = registrationAccessToken
		if err := h.store.UpdateClient(ctx, client); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrServerError, err)
		}

		registrationClientURI = h.server.config.GetBaseURL() + "/register/" + clientID
	}

	// Build response
	resp := &ClientRegistrationResponse{
		ClientID:                clientID,
		ClientSecret:            clientSecret,
		ClientSecretExpiresAt:   clientSecretExpiresAt,
		RegistrationAccessToken: registrationAccessToken,
		RegistrationClientURI:   registrationClientURI,
		RedirectURIs:            req.RedirectURIs,
		GrantTypes:              grantTypes,
		ApplicationType:         applicationType,
		Contacts:                req.Contacts,
		ClientName:              req.ClientName,
		LogoURI:                 req.LogoURI,
		ClientURI:               req.ClientURI,
		PolicyURI:               req.PolicyURI,
		TosURI:                  req.TosURI,
		JWKSURI:                 req.JWKSURI,
		JWKS:                    req.JWKS,
		SoftwareID:              req.SoftwareID,
		SoftwareVersion:         req.SoftwareVersion,
		TokenEndpointAuthMethod: req.TokenEndpointAuthMethod,
		Scope:                   strings.Join(scopes, " "),
		ResourceIndicators:      resources,
		MCPVersion:              req.MCPVersion,
		MCPTransportsSupported:  req.MCPTransportsSupported,
		ClientIDIssuedAt:        now.Unix(),
	}

	return resp, nil
}

// handleGetClient retrieves client configuration
func (h *DynamicRegistrationHandler) handleGetClient(w http.ResponseWriter, r *http.Request, clientID string) {
	client, err := h.store.GetClient(r.Context(), clientID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.writeError(w, http.StatusNotFound, ErrInvalidRequest, "Client not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, ErrServerError, "Failed to retrieve client")
		return
	}

	// Build response (don't include client_secret)
	resp := h.buildClientInfoResponse(client)
	h.writeJSON(w, http.StatusOK, resp)
}

// handleUpdateClient updates client configuration
func (h *DynamicRegistrationHandler) handleUpdateClient(w http.ResponseWriter, r *http.Request, clientID string) {
	var req ClientUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, ErrInvalidRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	req.ClientID = clientID

	// Retrieve existing client
	client, err := h.store.GetClient(r.Context(), clientID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			h.writeError(w, http.StatusNotFound, ErrInvalidRequest, "Client not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, ErrServerError, "Failed to retrieve client")
		return
	}

	// Update fields
	if len(req.RedirectURIs) > 0 {
		client.RedirectURIs = req.RedirectURIs
	}
	if len(req.GrantTypes) > 0 {
		client.GrantTypes = req.GrantTypes
	}
	if req.ClientName != "" {
		client.Name = req.ClientName
	}
	if req.Scope != "" {
		client.Scopes = parseScopes(req.Scope)
	}

	client.UpdatedAt = time.Now()

	if err := h.store.UpdateClient(r.Context(), client); err != nil {
		h.writeError(w, http.StatusInternalServerError, ErrServerError, "Failed to update client")
		return
	}

	resp := h.buildClientInfoResponse(client)
	h.writeJSON(w, http.StatusOK, resp)
}

// handleDeleteClient deletes a client
func (h *DynamicRegistrationHandler) handleDeleteClient(w http.ResponseWriter, r *http.Request, clientID string) {
	if err := h.store.DeleteClient(r.Context(), clientID); err != nil {
		if errors.Is(err, ErrNotFound) {
			h.writeError(w, http.StatusNotFound, ErrInvalidRequest, "Client not found")
			return
		}
		h.writeError(w, http.StatusInternalServerError, ErrServerError, "Failed to delete client")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// buildClientInfoResponse builds a client info response
func (h *DynamicRegistrationHandler) buildClientInfoResponse(client *Client) *ClientRegistrationResponse {
	applicationType, _ := client.Metadata["application_type"].(string)
	tokenEndpointAuthMethod, _ := client.Metadata["token_endpoint_auth_method"].(string)
	clientSecretExpiresAt, _ := client.Metadata["client_secret_expires_at"].(int64)

	return &ClientRegistrationResponse{
		ClientID:                client.ID,
		ClientSecretExpiresAt:   clientSecretExpiresAt,
		RedirectURIs:            client.RedirectURIs,
		GrantTypes:              client.GrantTypes,
		ApplicationType:         applicationType,
		ClientName:              client.Name,
		TokenEndpointAuthMethod: tokenEndpointAuthMethod,
		Scope:                   strings.Join(client.Scopes, " "),
		ResourceIndicators:      client.Resources,
		ClientIDIssuedAt:        client.CreatedAt.Unix(),
	}
}

// validateRegistrationAccessToken validates the registration access token
func (h *DynamicRegistrationHandler) validateRegistrationAccessToken(ctx context.Context, clientID, token string) error {
	client, err := h.store.GetClient(ctx, clientID)
	if err != nil {
		return err
	}

	storedToken, ok := client.Metadata["registration_access_token"].(string)
	if !ok || storedToken != token {
		return errors.New("invalid registration access token")
	}

	return nil
}

// Helper functions

// GenerateClientID generates a unique client ID
func GenerateClientID() (string, error) {
	return GenerateRandomString(32)
}

// GenerateClientSecret generates a secure client secret
func GenerateClientSecret() (string, error) {
	return GenerateRandomString(64)
}

// GenerateRegistrationAccessToken generates a registration access token
func GenerateRegistrationAccessToken() (string, error) {
	return GenerateRandomString(48)
}

// GenerateRandomString generates a cryptographically secure random string
func GenerateRandomString(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:length], nil
}

// DefaultRedirectURIValidator validates redirect URIs
func DefaultRedirectURIValidator(uri, applicationType string) error {
	// Web clients must use HTTPS
	if applicationType == "web" && !strings.HasPrefix(uri, "https://") {
		// Allow localhost for development
		if !strings.HasPrefix(uri, "http://localhost") && !strings.HasPrefix(uri, "http://127.0.0.1") {
			return fmt.Errorf("web clients must use HTTPS redirect URIs")
		}
	}

	// Native clients can use custom schemes
	if applicationType == "native" {
		if strings.HasPrefix(uri, "http://") && !strings.HasPrefix(uri, "http://localhost") && !strings.HasPrefix(uri, "http://127.0.0.1") {
			return fmt.Errorf("native clients cannot use http:// URIs except localhost")
		}
	}

	// Reject fragment components
	if strings.Contains(uri, "#") {
		return fmt.Errorf("redirect URIs must not contain fragment components")
	}

	return nil
}

// extractBearerToken extracts Bearer token from Authorization header
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return parts[1]
}

// extractClientIDFromPath extracts client ID from URL path
func extractClientIDFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) >= 2 && parts[0] == "register" {
		return parts[1]
	}
	return ""
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// HTTP response helpers

func (h *DynamicRegistrationHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *DynamicRegistrationHandler) writeError(w http.ResponseWriter, status int, errorCode, description string) {
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

func (h *DynamicRegistrationHandler) writeErrorFromErr(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	errorCode := ErrInvalidRequest
	description := err.Error()

	errStr := err.Error()
	if strings.Contains(errStr, ErrInvalidClient) {
		status = http.StatusUnauthorized
		errorCode = ErrInvalidClient
	} else if strings.Contains(errStr, ErrServerError) {
		status = http.StatusInternalServerError
		errorCode = ErrServerError
	}

	h.writeError(w, status, errorCode, description)
}
