package security

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// jwtClaims sengaja tidak memuat sub.
//
// Yang dibawa token hanya PENUNJUK sesi, bukan identitas. Siapa pemiliknya
// dijawab SessionStore, dan itu yang membuat sesi yang dihapus langsung
// mematikan token yang sudah beredar — selama identitas ada di dalam token,
// logout tidak dapat menyentuhnya sampai ia kedaluwarsa sendiri.
//
// Sebelumnya sub ada dan DIENKRIPSI AES-GCM supaya id penggunanya tidak terbaca
// dari token yang di-decode. Enkripsi itu, beserta kuncinya di environment,
// hilang bersama muatannya: yang tidak dibawa tidak perlu disembunyikan.
type jwtClaims struct {
	SessionID string `json:"sid,omitempty"`
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type JWTSigner struct {
	secret []byte
}

func NewJWTSigner(cfg config.Config) (output.TokenSigner, error) {
	return &JWTSigner{secret: []byte(cfg.JWTSecret)}, nil
}

func (s *JWTSigner) GenerateAccessToken(_ context.Context, claims output.TokenClaims, ttl time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		SessionID: claims.SessionID,
		TokenType: output.TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	return token.SignedString(s.secret)
}

func (s *JWTSigner) GenerateRefreshToken(_ context.Context, claims output.TokenClaims, ttl time.Duration) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		SessionID: claims.SessionID,
		TokenType: output.TokenTypeRefresh,
		RegisteredClaims: jwt.RegisteredClaims{
			// jti membuat dua refresh token untuk sesi yang sama tidak pernah
			// berupa byte yang identik, sehingga rotasi benar-benar menghasilkan
			// token baru walau detiknya sama.
			ID:        uuid.NewString(),
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
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, err
	}

	return &output.TokenClaims{
		SessionID: claims.SessionID,
		TokenType: claims.TokenType,
	}, nil
}
