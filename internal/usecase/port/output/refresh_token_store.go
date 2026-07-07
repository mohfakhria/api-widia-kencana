package output

import (
	"context"
	"time"
)

type RefreshTokenStore interface {
	Set(ctx context.Context, sessionID string, session RefreshSession, ttl time.Duration) error
	Get(ctx context.Context, sessionID string) (*RefreshSession, error)
	Delete(ctx context.Context, userID, sessionID string) error
	DeleteAll(ctx context.Context, userID string) error
	Enabled() bool
}

type RefreshSession struct {
	UserID    string `json:"user_id"`
	TokenHash string `json:"token_hash"`
}
