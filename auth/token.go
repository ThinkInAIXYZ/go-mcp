package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// TokenGenerator handles token generation and validation
type TokenGenerator struct {
	// JWT signing
	SigningKey []byte
	PrivateKey *rsa.PrivateKey // For RS256
	Algorithm  string          // "HS256" or "RS256"

	// Issuer identifier
	Issuer string
	// Key ID for JWKS
	KeyID string
	// Token expiration times
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	// Token rotation settings
	RotateRefreshTokens bool
}

// TokenClaims represents JWT claims for access tokens
type TokenClaims struct {
	jwt.RegisteredClaims
	ClientID string   `json:"client_id,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
	TokenID  string   `json:"jti,omitempty"`
}

// NewTokenGenerator creates a new token generator with default settings
func NewTokenGenerator(signingKey []byte, issuer string) *TokenGenerator {
	return &TokenGenerator{
		SigningKey:          signingKey,
		Issuer:              issuer,
		KeyID:               GenerateKeyID(signingKey),
		Algorithm:           "HS256",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     30 * 24 * time.Hour,
		RotateRefreshTokens: true,
	}
}

// NewTokenGeneratorWithRSA creates a token generator with RS256
func NewTokenGeneratorWithRSA(privateKey *rsa.PrivateKey, issuer string) *TokenGenerator {
	return &TokenGenerator{
		PrivateKey:          privateKey,
		Issuer:              issuer,
		KeyID:               GenerateRSAKeyID(&privateKey.PublicKey),
		Algorithm:           "RS256",
		AccessTokenTTL:      15 * time.Minute,
		RefreshTokenTTL:     30 * 24 * time.Hour,
		RotateRefreshTokens: true,
	}
}

// GenerateRandomToken generates a cryptographically secure random token
func GenerateRandomToken(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// GenerateAccessToken generates a new JWT access token with resource indicators
func (g *TokenGenerator) GenerateAccessToken(clientID, userID string, scopes []string, resources ...[]string) (*AccessToken, error) {
	now := time.Now()
	expiresAt := now.Add(g.AccessTokenTTL)

	// Generate token ID for tracking
	tokenID, err := GenerateRandomToken(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w", err)
	}

	// Flatten resources parameter
	var resourceList []string
	if len(resources) > 0 {
		resourceList = resources[0]
	}

	// RFC 8707: Set audience to resource indicators
	audience := jwt.ClaimStrings{clientID}
	if len(resourceList) > 0 {
		audience = make(jwt.ClaimStrings, len(resourceList))
		copy(audience, resourceList)
	}

	// Create JWT claims
	claims := TokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    g.Issuer,
			Subject:   userID,
			Audience:  audience,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ID:        tokenID,
		},
		ClientID: clientID,
		Scopes:   scopes,
		TokenID:  tokenID,
	}

	// Create and sign token based on algorithm
	var token *jwt.Token
	var tokenString string

	if g.Algorithm == "RS256" {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		token.Header["kid"] = g.KeyID
		tokenString, err = token.SignedString(g.PrivateKey)
	} else {
		token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		token.Header["kid"] = g.KeyID
		tokenString, err = token.SignedString(g.SigningKey)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &AccessToken{
		Token:     tokenString,
		TokenType: TokenTypeBearer,
		ClientID:  clientID,
		UserID:    userID,
		Scopes:    scopes,
		Resources: resourceList,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// GenerateRefreshToken generates a new refresh token
func (g *TokenGenerator) GenerateRefreshToken(clientID, userID string, scopes []string, resources ...[]string) (*RefreshToken, error) {
	now := time.Now()
	expiresAt := now.Add(g.RefreshTokenTTL)

	// Flatten resources parameter
	var resourceList []string
	if len(resources) > 0 {
		resourceList = resources[0]
	}

	// Generate opaque token
	tokenString, err := GenerateRandomToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	return &RefreshToken{
		Token:         tokenString,
		ClientID:      clientID,
		UserID:        userID,
		Scopes:        scopes,
		Resources:     resourceList,
		ExpiresAt:     expiresAt,
		CreatedAt:     now,
		RotationCount: 0,
		Revoked:       false,
	}, nil
}

// GenerateAuthorizationCode generates a new authorization code
func GenerateAuthorizationCode() (string, error) {
	return GenerateRandomToken(32)
}

// ValidateAccessToken validates and parses a JWT access token
func (g *TokenGenerator) ValidateAccessToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if g.Algorithm == "RS256" {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			// Return public key for verification
			return &g.PrivateKey.PublicKey, nil
		} else {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return g.SigningKey, nil
		}
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*TokenClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Verify issuer
	if claims.Issuer != g.Issuer {
		return nil, fmt.Errorf("invalid issuer")
	}

	return claims, nil
}

// ValidateTokenAudience validates that the token's audience matches the requested resource
func (g *TokenGenerator) ValidateTokenAudience(claims *TokenClaims, resource string) error {
	if resource == "" {
		return nil // No specific resource requested
	}

	for _, aud := range claims.Audience {
		if aud == resource {
			return nil
		}
	}

	return fmt.Errorf("token audience does not include requested resource: %s", resource)
}

// CreateTokenResponse creates a token response for the client
func CreateTokenResponse(accessToken *AccessToken, refreshToken *RefreshToken) *TokenResponse {
	response := &TokenResponse{
		AccessToken: accessToken.Token,
		TokenType:   string(accessToken.TokenType),
		ExpiresIn:   int64(time.Until(accessToken.ExpiresAt).Seconds()),
	}

	if refreshToken != nil {
		response.RefreshToken = refreshToken.Token
	}

	if len(accessToken.Scopes) > 0 {
		response.Scope = strings.Join(accessToken.Scopes, " ")
	}

	return response
}

// GenerateRSAKeyID creates a key ID from RSA public key
func GenerateRSAKeyID(publicKey *rsa.PublicKey) string {
	n := publicKey.N.Bytes()
	e := big.NewInt(int64(publicKey.E)).Bytes()
	combined := append(n, e...)
	return GenerateKeyID(combined)
}
