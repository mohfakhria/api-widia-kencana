package security

import (
	"context"
	"fmt"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/golang-jwt/jwt/v5"
)

type jwtClaims struct {
	jwt.RegisteredClaims
}

type JWTSigner struct {
	secret        []byte
	subjectCipher *SubjectCipher
}

func NewJWTSigner(cfg config.Config) (output.TokenSigner, error) {
	subjectCipher, err := NewSubjectCipher(cfg.JWTSubEncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("initialize JWT subject cipher: %w", err)
	}

	return &JWTSigner{
		secret:        []byte(cfg.JWTSecret),
		subjectCipher: subjectCipher,
	}, nil
}

func (s *JWTSigner) GenerateAccessToken(_ context.Context, claims output.TokenClaims, ttl time.Duration) (string, error) {
	encryptedSubject, err := s.subjectCipher.Encrypt(claims.Subject)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   encryptedSubject,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	return token.SignedString(s.secret)
}

func (s *JWTSigner) GenerateRefreshToken(_ context.Context, userID string, ttl time.Duration) (string, error) {
	encryptedSubject, err := s.subjectCipher.Encrypt(userID)
	if err != nil {
		return "", err
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwtClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   encryptedSubject,
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

	subject, err := s.subjectCipher.Decrypt(claims.Subject)
	if err != nil {
		return nil, err
	}

	return &output.TokenClaims{
		Subject: subject,
	}, nil
}
