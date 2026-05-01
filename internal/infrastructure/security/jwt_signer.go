package security

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/golang-jwt/jwt/v5"
)

type jwtClaims struct {
	Sub  string `json:"sub"`
	Name string `json:"name"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

type JWTSigner struct {
	secret []byte
}

func NewJWTSigner(cfg config.Config) output.TokenSigner {
	return &JWTSigner{secret: []byte(cfg.JWTSecret)}
}

func (s *JWTSigner) GenerateAccessToken(_ context.Context, claims output.TokenClaims, ttl time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		Sub:  claims.Subject,
		Name: claims.Name,
		Role: claims.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	return token.SignedString(s.secret)
}

func (s *JWTSigner) GenerateRefreshToken(_ context.Context, userID string, ttl time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		Sub: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	return token.SignedString(s.secret)
}

func (s *JWTSigner) ParseToken(_ context.Context, token string) (*output.TokenClaims, error) {
	claims := &jwtClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	return &output.TokenClaims{
		Subject: claims.Sub,
		Name:    claims.Name,
		Role:    claims.Role,
	}, nil
}
