package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
)

// PKCE (Proof Key for Code Exchange) support as required by OAuth 2.1

const (
	// CodeChallengeMethodPlain is not recommended but kept for compatibility
	CodeChallengeMethodPlain = "plain"
	// CodeChallengeMethodS256 is the recommended method
	CodeChallengeMethodS256 = "S256"

	// Code verifier requirements (RFC 7636)
	MinCodeVerifierLength = 43
	MaxCodeVerifierLength = 128
)

var (
	ErrInvalidCodeVerifier        = errors.New("auth: invalid code verifier")
	ErrInvalidCodeChallenge       = errors.New("auth: invalid code challenge")
	ErrInvalidCodeChallengeMethod = errors.New("auth: invalid code challenge method")
	ErrCodeChallengeMismatch      = errors.New("auth: code challenge verification failed")
	ErrPKCERequired               = errors.New("auth: PKCE is required for this client")
)

// codeVerifierRegex validates code verifier format (RFC 7636)
var codeVerifierRegex = regexp.MustCompile(`^[A-Za-z0-9\-._~]{43,128}$`)

// PKCEValidator handles PKCE validation
type PKCEValidator struct {
	// RequirePKCE forces all clients to use PKCE
	RequirePKCE bool
	// RequireS256 forces S256 method (recommended for OAuth 2.1)
	RequireS256 bool
}

// NewPKCEValidator creates a new PKCE validator with OAuth 2.1 defaults
func NewPKCEValidator() *PKCEValidator {
	return &PKCEValidator{
		RequirePKCE: true, // OAuth 2.1 requires PKCE
		RequireS256: true, // OAuth 2.1 recommends S256
	}
}

// GenerateCodeVerifier generates a cryptographically random code verifier
func GenerateCodeVerifier() (string, error) {
	// Generate 32 random bytes (256 bits)
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate code verifier: %w", err)
	}

	// Base64 URL encode without padding
	verifier := base64.RawURLEncoding.EncodeToString(b)
	return verifier, nil
}

// GenerateCodeChallenge generates a code challenge from a verifier
func GenerateCodeChallenge(verifier string, method string) (string, error) {
	if err := ValidateCodeVerifier(verifier); err != nil {
		return "", err
	}

	switch method {
	case CodeChallengeMethodPlain:
		return verifier, nil
	case CodeChallengeMethodS256:
		h := sha256.New()
		h.Write([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))
		return challenge, nil
	default:
		return "", ErrInvalidCodeChallengeMethod
	}
}

// ValidateCodeVerifier validates the format of a code verifier
func ValidateCodeVerifier(verifier string) error {
	if len(verifier) < MinCodeVerifierLength || len(verifier) > MaxCodeVerifierLength {
		return fmt.Errorf("%w: length must be between %d and %d",
			ErrInvalidCodeVerifier, MinCodeVerifierLength, MaxCodeVerifierLength)
	}

	if !codeVerifierRegex.MatchString(verifier) {
		return fmt.Errorf("%w: invalid characters", ErrInvalidCodeVerifier)
	}

	return nil
}

// ValidateCodeChallenge validates PKCE parameters during authorization
func (v *PKCEValidator) ValidateCodeChallenge(challenge, method string, isPublicClient bool) error {
	// OAuth 2.1 requires PKCE for public clients
	if isPublicClient && challenge == "" {
		return ErrPKCERequired
	}

	// If PKCE is required for all clients
	if v.RequirePKCE && challenge == "" {
		return ErrPKCERequired
	}

	// If no challenge provided and it's not required, skip validation
	if challenge == "" {
		return nil
	}

	// Validate challenge method
	if method == "" {
		method = CodeChallengeMethodPlain
	}

	if method != CodeChallengeMethodPlain && method != CodeChallengeMethodS256 {
		return ErrInvalidCodeChallengeMethod
	}

	// OAuth 2.1 recommends S256
	if v.RequireS256 && method != CodeChallengeMethodS256 {
		return fmt.Errorf("%w: S256 method required", ErrInvalidCodeChallengeMethod)
	}

	return nil
}

// VerifyCodeChallenge verifies the code verifier against the stored challenge
func VerifyCodeChallenge(verifier, challenge, method string) error {
	if err := ValidateCodeVerifier(verifier); err != nil {
		return err
	}

	if challenge == "" {
		// No challenge was stored, PKCE not used
		return nil
	}

	// Generate challenge from verifier
	computedChallenge, err := GenerateCodeChallenge(verifier, method)
	if err != nil {
		return err
	}

	// Compare challenges
	if computedChallenge != challenge {
		return ErrCodeChallengeMismatch
	}

	return nil
}

// GenerateCodeVerifierAndChallenge is a helper that generates both verifier and challenge
func GenerateCodeVerifierAndChallenge() (verifier, challenge string, err error) {
	verifier, err = GenerateCodeVerifier()
	if err != nil {
		return "", "", err
	}

	challenge, err = GenerateCodeChallenge(verifier, CodeChallengeMethodS256)
	if err != nil {
		return "", "", err
	}

	return verifier, challenge, nil
}
