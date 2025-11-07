package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
)

// JWK represents a JSON Web Key
type JWK struct {
	Kty string `json:"kty"`           // Key Type
	Use string `json:"use,omitempty"` // Public Key Use
	Kid string `json:"kid,omitempty"` // Key ID
	Alg string `json:"alg,omitempty"` // Algorithm
	N   string `json:"n,omitempty"`   // RSA Modulus (for RSA keys)
	E   string `json:"e,omitempty"`   // RSA Exponent (for RSA keys)
	K   string `json:"k,omitempty"`   // Symmetric key (for oct keys)
}

// JWKSet represents a JSON Web Key Set
type JWKSet struct {
	Keys []JWK `json:"keys"`
}

// JWKSProvider manages JWKS endpoint
type JWKSProvider struct {
	privateKey *rsa.PrivateKey // For RS256
	publicKey  *rsa.PublicKey  // For RS256
	signingKey []byte          // For HS256 (backward compatibility)
	keyID      string
	jwks       *JWKSet
	algorithm  string // "RS256" or "HS256"
}

// NewJWKSProvider creates a new JWKS provider
// For backward compatibility, it uses HS256 by default
func NewJWKSProvider(signingKey []byte, keyID string) *JWKSProvider {
	provider := &JWKSProvider{
		signingKey: signingKey,
		keyID:      keyID,
		algorithm:  "HS256",
	}
	provider.generateJWKS()
	return provider
}

// NewJWKSProviderWithRSA creates a JWKS provider with RSA support
func NewJWKSProviderWithRSA(keyID string) (*JWKSProvider, error) {
	// Generate RSA key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	provider := &JWKSProvider{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		keyID:      keyID,
		algorithm:  "RS256",
	}
	provider.generateJWKS()
	return provider, nil
}

// NewJWKSProviderWithRSAKey creates a JWKS provider with existing RSA key
func NewJWKSProviderWithRSAKey(privateKey *rsa.PrivateKey, keyID string) *JWKSProvider {
	provider := &JWKSProvider{
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		keyID:      keyID,
		algorithm:  "RS256",
	}
	provider.generateJWKS()
	return provider
}

// generateJWKS creates the JWKS document
func (jp *JWKSProvider) generateJWKS() {
	if jp.algorithm == "RS256" && jp.publicKey != nil {
		jp.jwks = &JWKSet{
			Keys: []JWK{
				{
					Kty: "RSA",
					Use: "sig",
					Kid: jp.keyID,
					Alg: "RS256",
					N:   base64.RawURLEncoding.EncodeToString(jp.publicKey.N.Bytes()),
					E:   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(jp.publicKey.E)).Bytes()),
				},
			},
		}
	} else {
		jp.jwks = &JWKSet{
			Keys: []JWK{}, // empty for HS256
		}
	}
}

// GetJWKS returns the JWKS document
func (jp *JWKSProvider) GetJWKS() *JWKSet {
	return jp.jwks
}

// GetPrivateKey returns the RSA private key (for signing)
func (jp *JWKSProvider) GetPrivateKey() *rsa.PrivateKey {
	return jp.privateKey
}

// GetAlgorithm returns the signing algorithm
func (jp *JWKSProvider) GetAlgorithm() string {
	return jp.algorithm
}

// ServeHTTP handles the JWKS endpoint
func (jp *JWKSProvider) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if jp.algorithm == "HS256" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "jwks_not_available",
			"error_description": "JWKS endpoint is not available for HMAC-based tokens. Use token introspection endpoint instead.",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	json.NewEncoder(w).Encode(jp.jwks)
}

// GenerateKeyID creates a key ID from the signing key
func GenerateKeyID(signingKey []byte) string {
	h := hmac.New(sha256.New, []byte("key-id-salt"))
	h.Write(signingKey)
	hash := h.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(hash[:16])
}
