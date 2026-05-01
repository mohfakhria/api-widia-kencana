package output

import (
	"context"
	"time"
)

type RefreshTokenStore interface {
	Set(ctx context.Context, userID, token string, ttl time.Duration) error
	Get(ctx context.Context, userID string) (string, error)
	Delete(ctx context.Context, userID string) error
	Enabled() bool
}
