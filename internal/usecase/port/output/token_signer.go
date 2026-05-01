package output

import (
	"context"
	"time"
)

type TokenClaims struct {
	Subject string
	Name    string
	Role    string
}

type TokenSigner interface {
	GenerateAccessToken(ctx context.Context, claims TokenClaims, ttl time.Duration) (string, error)
	GenerateRefreshToken(ctx context.Context, userID string, ttl time.Duration) (string, error)
	ParseToken(ctx context.Context, token string) (*TokenClaims, error)
}
