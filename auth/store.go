package auth

import (
	"context"
	"errors"
)

var (
	ErrNotFound      = errors.New("auth: entity not found")
	ErrAlreadyExists = errors.New("auth: entity already exists")
	ErrExpired       = errors.New("auth: entity expired")
	ErrRevoked       = errors.New("auth: entity revoked")
	ErrInvalidStore  = errors.New("auth: invalid store operation")
)

// Store defines the interface for OAuth 2.1 data persistence
type Store interface {
	ClientStore
	AuthorizationCodeStore
	TokenStore
}

// ClientStore manages OAuth clients
type ClientStore interface {
	// GetClient retrieves a client by ID
	GetClient(ctx context.Context, clientID string) (*Client, error)

	// CreateClient creates a new client
	CreateClient(ctx context.Context, client *Client) error

	// UpdateClient updates an existing client
	UpdateClient(ctx context.Context, client *Client) error

	// DeleteClient deletes a client by ID
	DeleteClient(ctx context.Context, clientID string) error

	// ValidateClientSecret validates client credentials
	ValidateClientSecret(ctx context.Context, clientID, secret string) error
}

// AuthorizationCodeStore manages authorization codes
type AuthorizationCodeStore interface {
	// SaveAuthorizationCode saves an authorization code
	SaveAuthorizationCode(ctx context.Context, code *AuthorizationCode) error

	// GetAuthorizationCode retrieves an authorization code
	GetAuthorizationCode(ctx context.Context, code string) (*AuthorizationCode, error)

	// InvalidateAuthorizationCode marks a code as used
	InvalidateAuthorizationCode(ctx context.Context, code string) error

	// CleanupExpiredAuthorizationCodes removes expired codes
	CleanupExpiredAuthorizationCodes(ctx context.Context) error
}

// TokenStore manages access and refresh tokens
type TokenStore interface {
	// SaveAccessToken saves an access token
	SaveAccessToken(ctx context.Context, token *AccessToken) error

	// GetAccessToken retrieves an access token
	GetAccessToken(ctx context.Context, token string) (*AccessToken, error)

	// RevokeAccessToken revokes an access token
	RevokeAccessToken(ctx context.Context, token string) error

	// SaveRefreshToken saves a refresh token
	SaveRefreshToken(ctx context.Context, token *RefreshToken) error

	// GetRefreshToken retrieves a refresh token
	GetRefreshToken(ctx context.Context, token string) (*RefreshToken, error)

	// RevokeRefreshToken revokes a refresh token
	RevokeRefreshToken(ctx context.Context, token string) error

	// RotateRefreshToken creates a new refresh token and revokes the old one
	RotateRefreshToken(ctx context.Context, oldToken string, newToken *RefreshToken) error

	// CleanupExpiredTokens removes expired tokens
	CleanupExpiredTokens(ctx context.Context) error
}
