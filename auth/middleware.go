package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

// Middleware provides OAuth 2.1 authentication for MCP server
type Middleware struct {
	server         *Server
	tokenGenerator *TokenGenerator
	store          Store
	realm          string // RFC 6750: realm for WWW-Authenticate

	// Optional custom validators
	scopeValidator  func(context.Context, []string, string) bool
	userIDExtractor func(context.Context) (string, error)
}

// MiddlewareConfig configures OAuth middleware
type MiddlewareConfig struct {
	// Required scopes for all requests
	RequiredScopes []string
	// Optional scope validator
	ScopeValidator func(context.Context, []string, string) bool
	// Optional user ID extractor from context
	UserIDExtractor func(context.Context) (string, error)
	// Realm for WWW-Authenticate header
	Realm string
}

// NewMiddleware creates a new OAuth middleware
func NewMiddleware(server *Server) *Middleware {
	return &Middleware{
		server:         server,
		tokenGenerator: server.tokenGenerator,
		store:          server.store,
		realm:          "MCP Server",
	}
}

// SetRealm sets the realm for WWW-Authenticate header
func (m *Middleware) SetRealm(realm string) {
	m.realm = realm
}

// HTTPMiddleware returns an HTTP middleware that validates OAuth tokens
func (m *Middleware) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Extract token from Authorization header
		token := m.extractTokenFromHeader(r)
		if token == "" {
			m.writeUnauthorizedError(w, "invalid_request", "Missing or invalid authorization header")
			return
		}

		// Validate token
		claims, err := m.tokenGenerator.ValidateAccessToken(token)
		if err != nil {
			m.writeUnauthorizedError(w, "invalid_token", "The access token is invalid or expired")
			return
		}

		// Verify token exists in store (not revoked)
		accessToken, err := m.store.GetAccessToken(ctx, token)
		if err != nil {
			m.writeUnauthorizedError(w, "invalid_token", "Token not found or has been revoked")
			return
		}

		// RFC 8707: Validate resource if specified in request
		requestedResource := r.Header.Get("X-Resource-Indicator")
		if requestedResource != "" {
			if err := m.tokenGenerator.ValidateTokenAudience(claims, requestedResource); err != nil {
				m.writeInsufficientScopeError(w, "The access token does not have permission for the requested resource")
				return
			}
		}

		// Add OAuth context to request
		ctx = WithOAuthContext(ctx, &OAuthContext{
			ClientID:    claims.ClientID,
			UserID:      claims.Subject,
			Scopes:      claims.Scopes,
			AccessToken: accessToken,
		})

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MCPToolMiddleware is a generic function that accepts any type with the specified underlying function signature.
func MCPToolMiddleware[H ~func(context.Context, *protocol.CallToolRequest) (*protocol.CallToolResult, error)](
	m *Middleware, config *MiddlewareConfig,
) func(H) H {
	return func(next H) H {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			oauthCtx := GetOAuthContext(ctx)
			if oauthCtx == nil {
				return nil, fmt.Errorf("unauthorized: no OAuth context")
			}

			if config != nil && len(config.RequiredScopes) > 0 {
				if !m.hasRequiredScopes(oauthCtx.Scopes, config.RequiredScopes) {
					return nil, fmt.Errorf("insufficient_scope: required scopes %v", config.RequiredScopes)
				}
			}

			if config != nil && config.ScopeValidator != nil {
				if !config.ScopeValidator(ctx, oauthCtx.Scopes, req.Name) {
					return nil, fmt.Errorf("insufficient_scope: scope validation failed for tool: %s", req.Name)
				}
			}

			return next(ctx, req)
		}
	}
}

// ScopeBasedToolMiddleware is a generic function that accepts any type with the specified underlying function signature.
func ScopeBasedToolMiddleware[H ~func(context.Context, *protocol.CallToolRequest) (*protocol.CallToolResult, error)](
	m *Middleware, toolScopes map[string][]string,
) func(H) H {
	return func(next H) H {
		return func(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
			oauthCtx := GetOAuthContext(ctx)
			if oauthCtx == nil {
				return nil, fmt.Errorf("unauthorized: no OAuth context")
			}

			requiredScopes, ok := toolScopes[req.Name]
			if ok && !m.hasRequiredScopes(oauthCtx.Scopes, requiredScopes) {
				return nil, fmt.Errorf("insufficient_scope: tool '%s' requires scopes %v, have %v",
					req.Name, requiredScopes, oauthCtx.Scopes)
			}

			return next(ctx, req)
		}
	}
}

// extractTokenFromHeader extracts Bearer token from Authorization header
func (m *Middleware) extractTokenFromHeader(r *http.Request) string {
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

// hasRequiredScopes checks if user has all required scopes
func (m *Middleware) hasRequiredScopes(userScopes, requiredScopes []string) bool {
	scopeMap := make(map[string]bool)
	for _, scope := range userScopes {
		scopeMap[scope] = true
	}

	for _, required := range requiredScopes {
		if !scopeMap[required] {
			return false
		}
	}

	return true
}

// writeUnauthorizedError writes a 401 Unauthorized response with WWW-Authenticate header (RFC 6750)
func (m *Middleware) writeUnauthorizedError(w http.ResponseWriter, errorCode, description string) {
	// RFC 6750 Section 3: WWW-Authenticate header format
	wwwAuth := fmt.Sprintf(`Bearer realm="%s"`, m.realm)
	if errorCode != "" {
		wwwAuth += fmt.Sprintf(`, error="%s"`, errorCode)
	}
	if description != "" {
		wwwAuth += fmt.Sprintf(`, error_description="%s"`, escapeQuotes(description))
	}

	w.Header().Set("WWW-Authenticate", wwwAuth)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	fmt.Fprintf(w, `{"error":"%s","error_description":"%s"}`,
		errorCode, escapeQuotes(description))
}

// writeInsufficientScopeError writes a 403 Forbidden response with WWW-Authenticate header
func (m *Middleware) writeInsufficientScopeError(w http.ResponseWriter, description string) {
	wwwAuth := fmt.Sprintf(`Bearer realm="%s", error="insufficient_scope"`, m.realm)
	if description != "" {
		wwwAuth += fmt.Sprintf(`, error_description="%s"`, escapeQuotes(description))
	}

	w.Header().Set("WWW-Authenticate", wwwAuth)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)

	fmt.Fprintf(w, `{"error":"insufficient_scope","error_description":"%s"}`,
		escapeQuotes(description))
}

// escapeQuotes escapes double quotes for use in header values
func escapeQuotes(s string) string {
	return strings.ReplaceAll(s, `"`, `\"`)
}

// OAuthContext holds OAuth information in context
type OAuthContext struct {
	ClientID    string
	UserID      string
	Scopes      []string
	AccessToken *AccessToken
}

type oauthContextKey struct{}

// WithOAuthContext adds OAuth context to the context
func WithOAuthContext(ctx context.Context, oauthCtx *OAuthContext) context.Context {
	return context.WithValue(ctx, oauthContextKey{}, oauthCtx)
}

// GetOAuthContext retrieves OAuth context from context
func GetOAuthContext(ctx context.Context) *OAuthContext {
	value := ctx.Value(oauthContextKey{})
	if value == nil {
		return nil
	}
	return value.(*OAuthContext)
}

// GetClientID is a helper to get client ID from context
func GetClientID(ctx context.Context) string {
	if oauthCtx := GetOAuthContext(ctx); oauthCtx != nil {
		return oauthCtx.ClientID
	}
	return ""
}

// GetUserID is a helper to get user ID from context
func GetUserID(ctx context.Context) string {
	if oauthCtx := GetOAuthContext(ctx); oauthCtx != nil {
		return oauthCtx.UserID
	}
	return ""
}

// GetScopes is a helper to get scopes from context
func GetScopes(ctx context.Context) []string {
	if oauthCtx := GetOAuthContext(ctx); oauthCtx != nil {
		return oauthCtx.Scopes
	}
	return nil
}

// HasScope checks if context has a specific scope
func HasScope(ctx context.Context, scope string) bool {
	scopes := GetScopes(ctx)
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope checks if context has any of the specified scopes
func HasAnyScope(ctx context.Context, requiredScopes ...string) bool {
	userScopes := GetScopes(ctx)
	scopeMap := make(map[string]bool)
	for _, scope := range userScopes {
		scopeMap[scope] = true
	}

	for _, required := range requiredScopes {
		if scopeMap[required] {
			return true
		}
	}
	return false
}

// HasAllScopes checks if context has all of the specified scopes
func HasAllScopes(ctx context.Context, requiredScopes ...string) bool {
	userScopes := GetScopes(ctx)
	scopeMap := make(map[string]bool)
	for _, scope := range userScopes {
		scopeMap[scope] = true
	}

	for _, required := range requiredScopes {
		if !scopeMap[required] {
			return false
		}
	}
	return true
}
