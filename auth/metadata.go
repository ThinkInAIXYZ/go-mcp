package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
)

// AuthorizationServerMetadata represents OAuth 2.0 Authorization Server Metadata (RFC 8414)
type AuthorizationServerMetadata struct {
	// REQUIRED: The authorization server's issuer identifier
	Issuer string `json:"issuer"`

	// REQUIRED: URL of the authorization endpoint
	AuthorizationEndpoint string `json:"authorization_endpoint"`

	// REQUIRED: URL of the token endpoint
	TokenEndpoint string `json:"token_endpoint"`

	// OPTIONAL: URL of the JWK Set document
	JWKSURI string `json:"jwks_uri,omitempty"`

	// OPTIONAL: URL of the Dynamic Client Registration endpoint
	RegistrationEndpoint string `json:"registration_endpoint,omitempty"`

	// OPTIONAL: JSON array of scope values supported
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// REQUIRED: JSON array of response_type values supported
	ResponseTypesSupported []string `json:"response_types_supported"`

	// OPTIONAL: JSON array of response_mode values supported
	ResponseModesSupported []string `json:"response_modes_supported,omitempty"`

	// OPTIONAL: JSON array of grant types supported
	GrantTypesSupported []string `json:"grant_types_supported,omitempty"`

	// OPTIONAL: JSON array of client authentication methods supported
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`

	// OPTIONAL: JSON array of signing algorithms supported
	TokenEndpointAuthSigningAlgValuesSupported []string `json:"token_endpoint_auth_signing_alg_values_supported,omitempty"`

	// OPTIONAL: URL of the revocation endpoint
	RevocationEndpoint string `json:"revocation_endpoint,omitempty"`

	// OPTIONAL: Client authentication methods supported by revocation endpoint
	RevocationEndpointAuthMethodsSupported []string `json:"revocation_endpoint_auth_methods_supported,omitempty"`

	// OPTIONAL: URL of the introspection endpoint
	IntrospectionEndpoint string `json:"introspection_endpoint,omitempty"`

	// OPTIONAL: Client authentication methods supported by introspection endpoint
	IntrospectionEndpointAuthMethodsSupported []string `json:"introspection_endpoint_auth_methods_supported,omitempty"`

	// OPTIONAL: JSON array of PKCE code challenge methods supported
	CodeChallengeMethodsSupported []string `json:"code_challenge_methods_supported,omitempty"`

	// RFC 8707: Resource Indicators
	// OPTIONAL: Boolean indicating support for resource indicators
	ResourceIndicatorsSupported bool `json:"resource_indicators_supported,omitempty"`

	// MCP-specific extensions
	MCPVersion             string   `json:"mcp_version,omitempty"`
	MCPTransportsSupported []string `json:"mcp_transports_supported,omitempty"`
}

// ProtectedResourceMetadata represents OAuth 2.0 Protected Resource Metadata (RFC 9728)
// This metadata helps MCP clients discover how to access protected resources
// NOTE: RFC 9728 is NOT required by MCP spec but included for completeness
type ProtectedResourceMetadata struct {
	// REQUIRED: The protected resource identifier (URI)
	Resource string `json:"resource"`

	// REQUIRED: Array of authorization server identifiers that can issue tokens for this resource
	AuthorizationServers []string `json:"authorization_servers"`

	// OPTIONAL: Array of bearer token usage methods supported (e.g., "header", "body", "query")
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`

	// OPTIONAL: Array of signing algorithms for JWS access tokens
	ResourceSigningAlgValuesSupported []string `json:"resource_signing_alg_values_supported,omitempty"`

	// OPTIONAL: Array of encryption algorithms for JWE access tokens
	ResourceEncryptionAlgValuesSupported []string `json:"resource_encryption_alg_values_supported,omitempty"`

	// OPTIONAL: Array of encryption encoding algorithms for JWE access tokens
	ResourceEncryptionEncValuesSupported []string `json:"resource_encryption_enc_values_supported,omitempty"`

	// OPTIONAL: Scopes supported by this resource
	ScopesSupported []string `json:"scopes_supported,omitempty"`

	// OPTIONAL: URL of the resource's documentation
	ResourceDocumentation string `json:"resource_documentation,omitempty"`

	// MCP-specific extensions
	MCPCapabilities *MCPResourceCapabilities `json:"mcp_capabilities,omitempty"`
}

// MCPResourceCapabilities describes MCP-specific capabilities for a protected resource
type MCPResourceCapabilities struct {
	Tools     *MCPToolsCapability     `json:"tools,omitempty"`
	Prompts   *MCPPromptsCapability   `json:"prompts,omitempty"`
	Resources *MCPResourcesCapability `json:"resources,omitempty"`
}

type MCPToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type MCPPromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

type MCPResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// MetadataProvider generates authorization server and protected resource metadata
type MetadataProvider struct {
	config                    *ServerConfig
	authorizationBaseURL      string // Derived from MCP server URL per spec
	authServerMetadata        *AuthorizationServerMetadata
	protectedResourceMetadata *ProtectedResourceMetadata
	enableProtectedResourceMD bool
	logger                    Logger
}

// NewMetadataProvider creates a new metadata provider
// Automatically derives authorization base URL from MCP server URL per MCP spec
func NewMetadataProvider(config *ServerConfig) (*MetadataProvider, error) {
	mp := &MetadataProvider{
		config:                    config,
		enableProtectedResourceMD: true, // Enable by default for MCP compliance
		logger:                    config.Logger,
	}

	// Derive authorization base URL from MCP server URL
	// MCP spec: "The authorization base URL MUST be determined from the MCP server URL
	// by discarding any existing path component"
	baseURL, err := deriveAuthorizationBaseURL(config.MCPServerURL)
	if err != nil {
		return nil, err
	}
	mp.authorizationBaseURL = baseURL

	mp.generateAuthServerMetadata()
	mp.generateProtectedResourceMetadata()
	return mp, nil
}

// deriveAuthorizationBaseURL implements MCP spec requirement:
// "discarding any existing path component"
func deriveAuthorizationBaseURL(mcpServerURL string) (string, error) {
	if mcpServerURL == "" {
		return "", ErrInvalidServerURL
	}

	parsedURL, err := url.Parse(mcpServerURL)
	if err != nil {
		return "", err
	}

	// Discard path, query, and fragment per MCP spec
	parsedURL.Path = ""
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	return parsedURL.String(), nil
}

// SetProtectedResourceMetadataEnabled enables or disables protected resource metadata
func (mp *MetadataProvider) SetProtectedResourceMetadataEnabled(enabled bool) {
	mp.enableProtectedResourceMD = enabled
}

// generateAuthServerMetadata builds the authorization server metadata document
func (mp *MetadataProvider) generateAuthServerMetadata() {
	grantTypes := make([]string, len(mp.config.SupportedGrantTypes))
	for i, gt := range mp.config.SupportedGrantTypes {
		grantTypes[i] = string(gt)
	}

	mp.authServerMetadata = &AuthorizationServerMetadata{
		Issuer:                mp.config.Issuer,
		AuthorizationEndpoint: mp.authorizationBaseURL + "/oauth/authorize",
		TokenEndpoint:         mp.authorizationBaseURL + "/oauth/token",
		RevocationEndpoint:    mp.authorizationBaseURL + "/oauth/revoke",
		IntrospectionEndpoint: mp.authorizationBaseURL + "/oauth/introspect",
		JWKSURI:               mp.authorizationBaseURL + "/.well-known/jwks.json",
		RegistrationEndpoint:  mp.authorizationBaseURL + "/register",

		ResponseTypesSupported: []string{"code"},
		ResponseModesSupported: []string{"query"},
		GrantTypesSupported:    grantTypes,

		TokenEndpointAuthMethodsSupported: []string{
			"client_secret_basic",
			"none", // For public clients
		},

		RevocationEndpointAuthMethodsSupported: []string{
			"client_secret_basic",
		},

		IntrospectionEndpointAuthMethodsSupported: []string{
			"client_secret_basic",
		},

		CodeChallengeMethodsSupported: []string{"S256"},

		// RFC 8707 support
		ResourceIndicatorsSupported: true,

		// MCP-specific
		MCPVersion:             CurrentMCPVersion,
		MCPTransportsSupported: []string{"sse", "stdio"},

		ScopesSupported: []string{
			"read",
			"write",
			"tools:list",
			"tools:execute",
			"prompts:list",
			"prompts:execute",
			"resources:list",
			"resources:read",
		},
	}
}

// generateProtectedResourceMetadata builds the protected resource metadata (RFC 9728)
func (mp *MetadataProvider) generateProtectedResourceMetadata() {
	// Determine resource identifier (typically the MCP server's base URI)
	resourceURI := mp.config.Issuer
	if len(mp.config.SupportedResources) > 0 {
		resourceURI = mp.config.SupportedResources[0]
	}

	mp.protectedResourceMetadata = &ProtectedResourceMetadata{
		Resource: resourceURI,
		AuthorizationServers: []string{
			mp.config.Issuer, // This MCP server acts as its own authorization server
		},
		BearerMethodsSupported: []string{
			"header", // Authorization: Bearer <token> (MCP spec requirement)
		},
		ResourceSigningAlgValuesSupported: []string{
			"RS256", // RSA with SHA-256
			"HS256", // HMAC with SHA-256
		},
		ScopesSupported: []string{
			"read",
			"write",
			"tools:list",
			"tools:execute",
			"prompts:list",
			"prompts:execute",
			"resources:list",
			"resources:read",
		},
		ResourceDocumentation: mp.authorizationBaseURL + "/docs",
		MCPCapabilities: &MCPResourceCapabilities{
			Tools: &MCPToolsCapability{
				ListChanged: true,
			},
			Prompts: &MCPPromptsCapability{
				ListChanged: true,
			},
			Resources: &MCPResourcesCapability{
				Subscribe:   true,
				ListChanged: true,
			},
		},
	}
}

// GetAuthServerMetadata returns the authorization server metadata document
func (mp *MetadataProvider) GetAuthServerMetadata() *AuthorizationServerMetadata {
	return mp.authServerMetadata
}

// GetProtectedResourceMetadata returns the protected resource metadata document
func (mp *MetadataProvider) GetProtectedResourceMetadata() *ProtectedResourceMetadata {
	return mp.protectedResourceMetadata
}

// GetAuthorizationBaseURL returns the derived authorization base URL
func (mp *MetadataProvider) GetAuthorizationBaseURL() string {
	return mp.authorizationBaseURL
}

// ServeHTTP handles the authorization server metadata endpoint
// Endpoint: /.well-known/oauth-authorization-server
// MCP spec: "MCP clients MUST follow the OAuth 2.0 Authorization Server Metadata protocol"
func (mp *MetadataProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// MCP spec: Check MCP-Protocol-Version header
	mcpVersion := r.Header.Get(MCPProtocolVersionHeader)
	if mcpVersion != "" {
		if !mp.isCompatibleMCPVersion(mcpVersion) {
			mp.logger.Warnf("Client requested MCP version %s, server supports %s", mcpVersion, CurrentMCPVersion)
			// Continue serving - version mismatch is a warning, not an error
			// Clients should handle version differences gracefully
		}
	} else {
		mp.logger.Infof("Client did not provide MCP-Protocol-Version header")
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(mp.authServerMetadata)
}

// ServeProtectedResourceMetadata handles the protected resource metadata endpoint
// Endpoint: /.well-known/oauth-protected-resource
// NOTE: This is RFC 9728, not required by MCP spec but useful for advanced scenarios
func (mp *MetadataProvider) ServeProtectedResourceMetadata(w http.ResponseWriter, r *http.Request) {
	if !mp.enableProtectedResourceMD {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check MCP version
	mcpVersion := r.Header.Get(MCPProtocolVersionHeader)
	if mcpVersion != "" && !mp.isCompatibleMCPVersion(mcpVersion) {
		mp.logger.Warnf("Client requested MCP version %s for protected resource metadata", mcpVersion)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache for 1 hour
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(mp.protectedResourceMetadata)
}

// isCompatibleMCPVersion checks if the requested MCP version is compatible
func (mp *MetadataProvider) isCompatibleMCPVersion(requestedVersion string) bool {
	// For now, exact match required
	// In production, implement semantic versioning compatibility
	return requestedVersion == CurrentMCPVersion
}

// CustomizeProtectedResourceMetadata allows customization of protected resource metadata
func (mp *MetadataProvider) CustomizeProtectedResourceMetadata(
	customize func(*ProtectedResourceMetadata),
) {
	if mp.protectedResourceMetadata != nil {
		customize(mp.protectedResourceMetadata)
	}
}

// AddAuthorizationServer adds an additional authorization server to the protected resource metadata
// This is useful when the MCP server accepts tokens from external authorization servers
func (mp *MetadataProvider) AddAuthorizationServer(authServerURI string) {
	if mp.protectedResourceMetadata == nil {
		return
	}

	// Check if already exists
	for _, existing := range mp.protectedResourceMetadata.AuthorizationServers {
		if existing == authServerURI {
			return
		}
	}

	mp.protectedResourceMetadata.AuthorizationServers = append(
		mp.protectedResourceMetadata.AuthorizationServers,
		authServerURI,
	)
}

// Additional error for metadata provider
var ErrInvalidServerURL = errors.New("auth: invalid MCP server URL")
