package memory

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"

	"github.com/ThinkInAIXYZ/go-mcp/auth"
)

// Store implements an in-memory OAuth store for development and testing
type Store struct {
	clients       sync.Map // clientID -> *auth.Client
	authCodes     sync.Map // code -> *auth.AuthorizationCode
	accessTokens  sync.Map // token -> *auth.AccessToken
	refreshTokens sync.Map // token -> *auth.RefreshToken
	clientSecrets sync.Map // clientID -> hashedSecret
}

// NewStore creates a new in-memory store
func NewStore() *Store {
	return &Store{}
}

// Client operations

func (s *Store) GetClient(_ context.Context, clientID string) (*auth.Client, error) {
	value, ok := s.clients.Load(clientID)
	if !ok {
		return nil, auth.ErrNotFound
	}

	client := value.(*auth.Client)
	return client, nil
}

func (s *Store) CreateClient(_ context.Context, client *auth.Client) error {
	if client.ID == "" {
		return auth.ErrInvalidStore
	}

	_, loaded := s.clients.LoadOrStore(client.ID, client)
	if loaded {
		return auth.ErrAlreadyExists
	}

	// Hash and store client secret if present
	if client.Secret != "" {
		hashedSecret := hashSecret(client.Secret)
		s.clientSecrets.Store(client.ID, hashedSecret)
		client.Secret = ""
	}

	return nil
}

func (s *Store) UpdateClient(_ context.Context, client *auth.Client) error {
	if client.ID == "" {
		return auth.ErrInvalidStore
	}

	_, ok := s.clients.Load(client.ID)
	if !ok {
		return auth.ErrNotFound
	}

	// Update secret if provided
	if client.Secret != "" {
		hashedSecret := hashSecret(client.Secret)
		s.clientSecrets.Store(client.ID, hashedSecret)
		client.Secret = ""
	}

	client.UpdatedAt = time.Now()
	s.clients.Store(client.ID, client)

	return nil
}

func (s *Store) DeleteClient(_ context.Context, clientID string) error {
	s.clients.Delete(clientID)
	s.clientSecrets.Delete(clientID)
	return nil
}

func (s *Store) ValidateClientSecret(_ context.Context, clientID, secret string) error {
	value, ok := s.clientSecrets.Load(clientID)
	if !ok {
		return auth.ErrNotFound
	}

	hashedSecret := value.(string)
	providedHash := hashSecret(secret)

	// Use constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(hashedSecret), []byte(providedHash)) != 1 {
		return errors.New(auth.ErrInvalidClient)
	}

	return nil
}

// Authorization code operations

func (s *Store) SaveAuthorizationCode(_ context.Context, code *auth.AuthorizationCode) error {
	if code.Code == "" {
		return auth.ErrInvalidStore
	}

	s.authCodes.Store(code.Code, code)
	return nil
}

func (s *Store) GetAuthorizationCode(_ context.Context, code string) (*auth.AuthorizationCode, error) {
	value, ok := s.authCodes.Load(code)
	if !ok {
		return nil, auth.ErrNotFound
	}

	authCode := value.(*auth.AuthorizationCode)

	// Check expiration
	if time.Now().After(authCode.ExpiresAt) {
		return nil, auth.ErrExpired
	}

	// Check if already used
	if authCode.Used {
		return nil, auth.ErrRevoked
	}

	return authCode, nil
}

func (s *Store) InvalidateAuthorizationCode(_ context.Context, code string) error {
	value, ok := s.authCodes.Load(code)
	if !ok {
		return auth.ErrNotFound
	}

	authCode := value.(*auth.AuthorizationCode)
	authCode.Used = true
	s.authCodes.Store(code, authCode)

	return nil
}

func (s *Store) CleanupExpiredAuthorizationCodes(_ context.Context) error {
	now := time.Now()

	s.authCodes.Range(func(key, value interface{}) bool {
		code := value.(*auth.AuthorizationCode)
		if now.After(code.ExpiresAt) || code.Used {
			s.authCodes.Delete(key)
		}
		return true
	})

	return nil
}

// Access token operations

func (s *Store) SaveAccessToken(_ context.Context, token *auth.AccessToken) error {
	if token.Token == "" {
		return auth.ErrInvalidStore
	}

	s.accessTokens.Store(token.Token, token)
	return nil
}

func (s *Store) GetAccessToken(_ context.Context, token string) (*auth.AccessToken, error) {
	value, ok := s.accessTokens.Load(token)
	if !ok {
		return nil, auth.ErrNotFound
	}

	accessToken := value.(*auth.AccessToken)

	// Check expiration
	if time.Now().After(accessToken.ExpiresAt) {
		return nil, auth.ErrExpired
	}

	return accessToken, nil
}

func (s *Store) RevokeAccessToken(_ context.Context, token string) error {
	s.accessTokens.Delete(token)
	return nil
}

// Refresh token operations

func (s *Store) SaveRefreshToken(_ context.Context, token *auth.RefreshToken) error {
	if token.Token == "" {
		return auth.ErrInvalidStore
	}

	s.refreshTokens.Store(token.Token, token)
	return nil
}

func (s *Store) GetRefreshToken(_ context.Context, token string) (*auth.RefreshToken, error) {
	value, ok := s.refreshTokens.Load(token)
	if !ok {
		return nil, auth.ErrNotFound
	}

	refreshToken := value.(*auth.RefreshToken)

	// Check if revoked
	if refreshToken.Revoked {
		return nil, auth.ErrRevoked
	}

	// Check expiration
	if time.Now().After(refreshToken.ExpiresAt) {
		return nil, auth.ErrExpired
	}

	return refreshToken, nil
}

func (s *Store) RevokeRefreshToken(_ context.Context, token string) error {
	value, ok := s.refreshTokens.Load(token)
	if !ok {
		return auth.ErrNotFound
	}

	refreshToken := value.(*auth.RefreshToken)
	refreshToken.Revoked = true
	s.refreshTokens.Store(token, refreshToken)

	return nil
}

func (s *Store) RotateRefreshToken(ctx context.Context, oldToken string, newToken *auth.RefreshToken) error {
	// Revoke old token
	if err := s.RevokeRefreshToken(ctx, oldToken); err != nil {
		return err
	}

	// Save new token
	return s.SaveRefreshToken(ctx, newToken)
}

// Cleanup operations

func (s *Store) CleanupExpiredTokens(_ context.Context) error {
	now := time.Now()

	// Cleanup access tokens
	s.accessTokens.Range(func(key, value interface{}) bool {
		token := value.(*auth.AccessToken)
		if now.After(token.ExpiresAt) {
			s.accessTokens.Delete(key)
		}
		return true
	})

	// Cleanup refresh tokens
	s.refreshTokens.Range(func(key, value interface{}) bool {
		token := value.(*auth.RefreshToken)
		if now.After(token.ExpiresAt) || token.Revoked {
			s.refreshTokens.Delete(key)
		}
		return true
	})

	return nil
}

// hashSecret creates a SHA256 hash of the secret
func hashSecret(secret string) string {
	hash := sha256.Sum256([]byte(secret))
	return base64.StdEncoding.EncodeToString(hash[:])
}
