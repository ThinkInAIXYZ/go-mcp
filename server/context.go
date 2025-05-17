package server

import (
	"context"
	"errors"
)

type sessionIDKey struct{}

func setSessionIDToCtx(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, sessionID)
}

func GetSessionIDFromCtx(ctx context.Context) (string, error) {
	sessionID := ctx.Value(sessionIDKey{})
	if sessionID == nil {
		return "", errors.New("no session id found")
	}
	return sessionID.(string), nil
}
