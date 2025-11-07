package auth

import (
	"time"
)

// GrantType represents OAuth 2.1 grant types
type GrantType string

const (
	// OAuth 2.1 compliant grant types
	GrantTypeAuthorizationCode GrantType = "authorization_code"
	GrantTypeClientCredentials GrantType = "client_credentials"
	GrantTypeRefreshToken      GrantType = "refresh_token"
	GrantTypeDeviceCode        GrantType = "urn:ietf:params:oauth:grant-type:device_code"
)

// TokenType represents the type of token
type TokenType string

const (
	TokenTypeBearer TokenType = "Bearer"
)

// Client represents an OAuth 2.1 client application
type Client struct {
	ID           string                 `json:"client_id"`
	Secret       string                 `json:"client_secret,omitempty"` // Hashed
	Name         string                 `json:"name"`
	RedirectURIs []string               `json:"redirect_uris"`
	GrantTypes   []string               `json:"grant_types"`
	Scopes       []string               `json:"scopes"`
	Resources    []string               `json:"resources,omitempty"` // RFC 8707: Allowed resource indicators
	IsPublic     bool                   `json:"is_public"`           // Public clients must use PKCE
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// AuthorizationCode represents an OAuth 2.1 authorization code
type AuthorizationCode struct {
	Code                string                 `json:"code"`
	ClientID            string                 `json:"client_id"`
	UserID              string                 `json:"user_id"`
	RedirectURI         string                 `json:"redirect_uri"`
	Scopes              []string               `json:"scopes"`
	Resources           []string               `json:"resources,omitempty"` // RFC 8707
	CodeChallenge       string                 `json:"code_challenge"`
	CodeChallengeMethod string                 `json:"code_challenge_method"`
	ExpiresAt           time.Time              `json:"expires_at"`
	CreatedAt           time.Time              `json:"created_at"`
	Used                bool                   `json:"used"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// AccessToken represents an OAuth 2.1 access token
type AccessToken struct {
	Token        string                 `json:"token"`
	TokenType    TokenType              `json:"token_type"`
	ClientID     string                 `json:"client_id"`
	UserID       string                 `json:"user_id,omitempty"`
	Scopes       []string               `json:"scopes"`
	Resources    []string               `json:"resources,omitempty"` // RFC 8707: Token audience
	ExpiresAt    time.Time              `json:"expires_at"`
	CreatedAt    time.Time              `json:"created_at"`
	RefreshToken string                 `json:"refresh_token,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// RefreshToken represents an OAuth 2.1 refresh token
type RefreshToken struct {
	Token         string                 `json:"token"`
	ClientID      string                 `json:"client_id"`
	UserID        string                 `json:"user_id"`
	Scopes        []string               `json:"scopes"`
	Resources     []string               `json:"resources,omitempty"` // RFC 8707
	ExpiresAt     time.Time              `json:"expires_at"`
	CreatedAt     time.Time              `json:"created_at"`
	LastUsedAt    *time.Time             `json:"last_used_at,omitempty"`
	RotationCount int                    `json:"rotation_count"`
	Revoked       bool                   `json:"revoked"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// TokenResponse represents the OAuth 2.1 token endpoint response
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
}

// TokenRequest represents the OAuth 2.1 token endpoint request
type TokenRequest struct {
	GrantType    string   `json:"grant_type" form:"grant_type"`
	Code         string   `json:"code,omitempty" form:"code"`
	RedirectURI  string   `json:"redirect_uri,omitempty" form:"redirect_uri"`
	ClientID     string   `json:"client_id,omitempty" form:"client_id"`
	ClientSecret string   `json:"client_secret,omitempty" form:"client_secret"`
	RefreshToken string   `json:"refresh_token,omitempty" form:"refresh_token"`
	Scope        string   `json:"scope,omitempty" form:"scope"`
	Resource     []string `json:"resource,omitempty" form:"resource"` // RFC 8707: Can be repeated
	CodeVerifier string   `json:"code_verifier,omitempty" form:"code_verifier"`
	Username     string   `json:"username,omitempty" form:"username"` // Not in OAuth 2.1, kept for compatibility
	Password     string   `json:"password,omitempty" form:"password"` // Not in OAuth 2.1, kept for compatibility
}

// AuthorizationRequest represents the OAuth 2.1 authorization endpoint request
type AuthorizationRequest struct {
	ResponseType        string   `json:"response_type" form:"response_type"`
	ClientID            string   `json:"client_id" form:"client_id"`
	RedirectURI         string   `json:"redirect_uri" form:"redirect_uri"`
	Scope               string   `json:"scope,omitempty" form:"scope"`
	State               string   `json:"state,omitempty" form:"state"`
	Resource            []string `json:"resource,omitempty" form:"resource"` // RFC 8707
	CodeChallenge       string   `json:"code_challenge,omitempty" form:"code_challenge"`
	CodeChallengeMethod string   `json:"code_challenge_method,omitempty" form:"code_challenge_method"`
}

// IntrospectionRequest represents the token introspection request
type IntrospectionRequest struct {
	Token         string `json:"token" form:"token"`
	TokenTypeHint string `json:"token_type_hint,omitempty" form:"token_type_hint"`
}

// IntrospectionResponse represents the token introspection response
type IntrospectionResponse struct {
	Active    bool     `json:"active"`
	Scope     string   `json:"scope,omitempty"`
	ClientID  string   `json:"client_id,omitempty"`
	Username  string   `json:"username,omitempty"`
	TokenType string   `json:"token_type,omitempty"`
	ExpiresAt int64    `json:"exp,omitempty"`
	IssuedAt  int64    `json:"iat,omitempty"`
	Subject   string   `json:"sub,omitempty"`
	Audience  []string `json:"aud,omitempty"` // RFC 8707: Resource indicators
}

// ErrorResponse represents an OAuth 2.1 error response
type ErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
	ErrorURI         string `json:"error_uri,omitempty"`
}

// OAuth 2.1 error codes
const (
	ErrInvalidRequest          = "invalid_request"
	ErrInvalidClient           = "invalid_client"
	ErrInvalidGrant            = "invalid_grant"
	ErrUnauthorizedClient      = "unauthorized_client"
	ErrUnsupportedGrantType    = "unsupported_grant_type"
	ErrInvalidScope            = "invalid_scope"
	ErrInvalidTarget           = "invalid_target" // RFC 8707
	ErrAccessDenied            = "access_denied"
	ErrUnsupportedResponseType = "unsupported_response_type"
	ErrServerError             = "server_error"
	ErrTemporarilyUnavailable  = "temporarily_unavailable"
	ErrInvalidToken            = "invalid_token"
)

// MCP Protocol Constants
const (
	// CurrentMCPVersion is the MCP protocol version this implementation supports
	CurrentMCPVersion = "2024-11-05"

	// MCPProtocolVersionHeader is the header name for MCP protocol version
	MCPProtocolVersionHeader = "MCP-Protocol-Version"

	// MCP Resource URI schemes (RFC 8707)
	MCPResourceSchemeTools     = "mcp://tools"
	MCPResourceSchemePrompts   = "mcp://prompts"
	MCPResourceSchemeResources = "mcp://resources"
)

// DefaultMCPResources indicators
var DefaultMCPResources = []string{
	MCPResourceSchemeTools,
	MCPResourceSchemePrompts,
	MCPResourceSchemeResources,
}
