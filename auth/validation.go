package auth

import (
	"fmt"
	"net/url"
	"strings"
)

// validateRedirectURI validates that the redirect URI exactly matches one of the registered URIs
// OAuth 2.1 requires exact matching, no wildcards allowed
func validateRedirectURI(allowedURIs []string, requestedURI string) error {
	if requestedURI == "" {
		return fmt.Errorf("redirect_uri is required")
	}

	for _, allowed := range allowedURIs {
		if allowed == requestedURI {
			return nil
		}
	}

	return fmt.Errorf("%s: redirect_uri not registered for this client", ErrInvalidRequest)
}

// validateEndpointSecurity ensures HTTPS is used (except for localhost)
// OAuth 2.1 requirement: All endpoints must use HTTPS except localhost for development
// MCP spec: "Redirect URIs MUST be either localhost URLs or HTTPS URLs"
func validateEndpointSecurity(uri string) error {
	if uri == "" {
		return fmt.Errorf("URI cannot be empty")
	}

	parsedURL, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}

	// MCP spec: HTTPS or localhost required
	if parsedURL.Scheme == "https" {
		// HTTPS always allowed
		return nil
	}

	if parsedURL.Scheme == "http" {
		// HTTP only allowed for localhost
		if isLocalhost(parsedURL.Hostname()) {
			return nil
		}
		return fmt.Errorf("%s: HTTPS required for non-localhost endpoints (MCP spec requirement)", ErrInvalidRequest)
	}

	// Other schemes (e.g., custom schemes for native apps) may be allowed
	// but HTTPS/localhost validation is MCP requirement for web endpoints

	// Fragment components are not allowed in redirect URIs
	if parsedURL.Fragment != "" {
		return fmt.Errorf("%s: redirect URIs must not contain fragment components", ErrInvalidRequest)
	}

	return nil
}

// isLocalhost checks if a host is localhost or 127.0.0.1 or ::1 (IPv6)
// Updated to properly handle IPv6 localhost per MCP spec
func isLocalhost(host string) bool {
	if host == "" {
		return false
	}

	// Normalize host by removing brackets for IPv6
	normalizedHost := strings.Trim(host, "[]")

	// Check common localhost representations
	switch normalizedHost {
	case "localhost",
		"127.0.0.1",       // IPv4 loopback
		"::1",             // IPv6 loopback (short form)
		"0:0:0:0:0:0:0:1": // IPv6 loopback (long form)
		return true
	}

	// Check IPv6 loopback variations
	if strings.HasPrefix(normalizedHost, "::ffff:127.") {
		// IPv4-mapped IPv6 addresses (::ffff:127.0.0.1)
		return true
	}

	return false
}

// validateState ensures state parameter is present (CSRF protection)
// OAuth 2.1 strongly recommends state parameter for all authorization requests
func validateState(state string) error {
	if state == "" {
		return fmt.Errorf("%s: state parameter is required for CSRF protection", ErrInvalidRequest)
	}

	// State should be at least 8 characters for adequate entropy
	if len(state) < 8 {
		return fmt.Errorf("%s: state parameter too short (minimum 8 characters)", ErrInvalidRequest)
	}

	return nil
}

// validateScope validates requested scopes
func validateScope(requestedScopes []string, allowedScopes []string) error {
	if len(requestedScopes) == 0 {
		return nil // Empty scope is allowed
	}

	allowedMap := make(map[string]bool)
	for _, scope := range allowedScopes {
		allowedMap[scope] = true
	}

	for _, requested := range requestedScopes {
		if !allowedMap[requested] {
			return fmt.Errorf("%s: scope '%s' is not allowed", ErrInvalidScope, requested)
		}
	}

	return nil
}

// validateClientRedirectURIs validates all redirect URIs for a client during registration
func validateClientRedirectURIs(redirectURIs []string, applicationType string) error {
	if len(redirectURIs) == 0 {
		return fmt.Errorf("%s: at least one redirect_uri is required", ErrInvalidRequest)
	}

	for _, uri := range redirectURIs {
		if err := validateRedirectURIFormat(uri, applicationType); err != nil {
			return err
		}
	}

	return nil
}

// validateRedirectURIFormat validates the format of a redirect URI based on application type
// Updated to fully comply with MCP spec: "Redirect URIs MUST be either localhost URLs or HTTPS URLs"
func validateRedirectURIFormat(uri string, applicationType string) error {
	parsedURL, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("%s: invalid redirect URI format: %w", ErrInvalidRequest, err)
	}

	// Fragment components are forbidden
	if parsedURL.Fragment != "" {
		return fmt.Errorf("%s: redirect URIs must not contain fragment components", ErrInvalidRequest)
	}

	// MCP spec: "Redirect URIs MUST be either localhost URLs or HTTPS URLs"
	switch parsedURL.Scheme {
	case "https":
		// HTTPS always allowed per MCP spec
		return nil

	case "http":
		// HTTP only allowed for localhost per MCP spec
		hostname := parsedURL.Hostname()
		if !isLocalhost(hostname) {
			return fmt.Errorf("%s: HTTP redirect URIs only allowed for localhost (MCP spec requirement)", ErrInvalidRequest)
		}
		return nil

	default:
		// For native apps, custom schemes may be allowed
		if applicationType == "native" {
			// Custom schemes like myapp:// are allowed for native apps
			// but MCP spec primarily targets web/server scenarios
			return nil
		}

		return fmt.Errorf("%s: redirect URI must use HTTPS or localhost HTTP (MCP spec requirement)", ErrInvalidRequest)
	}
}

// parseScopes parses a space-separated scope string into a slice
func parseScopes(scopeString string) []string {
	if scopeString == "" {
		return []string{}
	}

	scopes := strings.Split(scopeString, " ")
	result := make([]string, 0, len(scopes))

	for _, scope := range scopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
