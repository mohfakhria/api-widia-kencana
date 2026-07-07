package usecase

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 30 * time.Second
	refreshTokenTTL = 7 * 24 * time.Hour
)

type authUseCase struct {
	userRepo    output.UserRepository
	tokenStore  output.RefreshTokenStore
	tokenSigner output.TokenSigner
}

func NewAuthUseCase(userRepo output.UserRepository, tokenStore output.RefreshTokenStore, tokenSigner output.TokenSigner) input.AuthUseCase {
	return &authUseCase{
		userRepo:    userRepo,
		tokenStore:  tokenStore,
		tokenSigner: tokenSigner,
	}
}

func (uc *authUseCase) Login(ctx context.Context, cmd input.LoginCommand) (*input.LoginResult, error) {
	user, err := uc.userRepo.FindByEmail(ctx, cmd.Email)
	if err != nil {
		return nil, domain.NewError(domain.ErrUnauthorized, "email not found")
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(cmd.Password)) != nil {
		return nil, domain.NewError(domain.ErrUnauthorized, "invalid password")
	}

	userID := strconv.FormatInt(user.ID, 10)
	sessionID := uuid.NewString()
	accessToken, err := uc.tokenSigner.GenerateAccessToken(ctx, output.TokenClaims{
		Subject:   userID,
		SessionID: sessionID,
	}, accessTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate access token")
	}

	refreshToken, err := uc.tokenSigner.GenerateRefreshToken(ctx, output.TokenClaims{
		Subject:   userID,
		SessionID: sessionID,
	}, refreshTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate refresh token")
	}

	if uc.tokenStore.Enabled() {
		if err := uc.tokenStore.Set(ctx, sessionID, output.RefreshSession{
			UserID:    userID,
			TokenHash: hashToken(refreshToken),
		}, refreshTokenTTL); err != nil {
			return nil, domain.NewError(domain.ErrInternalFailure, "Failed to store refresh token")
		}
	}

	return &input.LoginResult{
		AccessToken:     accessToken,
		AccessExpiredAt: time.Now().Add(accessTokenTTL).Unix(),
		RefreshToken:    refreshToken,
		RefreshTokenTTL: refreshTokenTTL,
	}, nil
}

func (uc *authUseCase) RefreshToken(ctx context.Context, cmd input.RefreshCommand) (*input.RefreshResult, error) {
	if !uc.tokenStore.Enabled() {
		return nil, domain.NewError(domain.ErrUnavailable, "Refresh token is disabled")
	}
	if cmd.RefreshToken == "" {
		return nil, domain.NewError(domain.ErrUnauthorized, "Missing refresh token")
	}

	claims, err := uc.tokenSigner.ParseToken(ctx, cmd.RefreshToken)
	if err != nil {
		return nil, domain.NewError(domain.ErrUnauthorized, "Invalid refresh token")
	}
	if claims.TokenType != output.TokenTypeRefresh || claims.SessionID == "" {
		return nil, domain.NewError(domain.ErrUnauthorized, "Invalid refresh token")
	}

	stored, err := uc.tokenStore.Get(ctx, claims.SessionID)
	if err != nil || stored.UserID != claims.Subject || !tokenHashMatches(stored.TokenHash, cmd.RefreshToken) {
		return nil, domain.NewError(domain.ErrUnauthorized, "Refresh token not valid or expired")
	}

	_, err = uc.userRepo.FindByID(ctx, claims.Subject)
	if err != nil {
		return nil, domain.NewError(domain.ErrUnauthorized, "User not found or deleted")
	}

	newAccess, err := uc.tokenSigner.GenerateAccessToken(ctx, output.TokenClaims{
		Subject:   claims.Subject,
		SessionID: claims.SessionID,
	}, accessTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate new access token")
	}

	newRefresh, err := uc.tokenSigner.GenerateRefreshToken(ctx, output.TokenClaims{
		Subject:   claims.Subject,
		SessionID: claims.SessionID,
	}, refreshTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate new refresh token")
	}

	if err := uc.tokenStore.Set(ctx, claims.SessionID, output.RefreshSession{
		UserID:    claims.Subject,
		TokenHash: hashToken(newRefresh),
	}, refreshTokenTTL); err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to store new refresh token")
	}

	return &input.RefreshResult{
		AccessToken:     newAccess,
		AccessExpiredAt: time.Now().Add(accessTokenTTL).Unix(),
		RefreshToken:    newRefresh,
		RefreshTokenTTL: refreshTokenTTL,
	}, nil
}

func (uc *authUseCase) Logout(ctx context.Context, cmd input.LogoutCommand) error {
	if !uc.tokenStore.Enabled() || cmd.RefreshToken == "" {
		return nil
	}

	claims, err := uc.tokenSigner.ParseToken(ctx, cmd.RefreshToken)
	if err != nil {
		return nil
	}
	if claims.TokenType != output.TokenTypeRefresh || claims.SessionID == "" {
		return nil
	}

	return uc.tokenStore.Delete(ctx, claims.Subject, claims.SessionID)
}

func (uc *authUseCase) LogoutAll(ctx context.Context, cmd input.LogoutAllCommand) error {
	if !uc.tokenStore.Enabled() {
		return nil
	}
	if cmd.UserID == "" {
		return domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}
	return uc.tokenStore.DeleteAll(ctx, cmd.UserID)
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func tokenHashMatches(storedHash, token string) bool {
	expectedHash := hashToken(token)
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(expectedHash)) == 1
}

func (uc *authUseCase) GetProfile(ctx context.Context, cmd input.GetProfileCommand) (*input.ProfileResult, error) {
	if cmd.UserID == "" {
		return nil, domain.NewError(domain.ErrUnauthorized, "Invalid or expired token")
	}
	user, err := uc.userRepo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return nil, domain.NewError(domain.ErrUnauthorized, "User not found or deleted")
	}

	return &input.ProfileResult{
		UserID: strconv.FormatInt(user.ID, 10),
		Name:   user.Name,
		Role:   user.Role,
	}, nil
}
