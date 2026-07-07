package output

import (
	"context"
	"time"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenClaims struct {
	Subject   string
	SessionID string
	TokenType string
}

type TokenSigner interface {
	GenerateAccessToken(ctx context.Context, claims TokenClaims, ttl time.Duration) (string, error)
	GenerateRefreshToken(ctx context.Context, claims TokenClaims, ttl time.Duration) (string, error)
	ParseToken(ctx context.Context, token string) (*TokenClaims, error)
}
