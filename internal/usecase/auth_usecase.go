package usecase

import (
	"context"
	"strconv"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"golang.org/x/crypto/bcrypt"
)

const (
	accessTokenTTL  = 10 * time.Minute
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
	accessToken, err := uc.tokenSigner.GenerateAccessToken(ctx, output.TokenClaims{
		Subject: userID,
	}, accessTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate access token")
	}

	refreshToken, err := uc.tokenSigner.GenerateRefreshToken(ctx, userID, refreshTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate refresh token")
	}

	if uc.tokenStore.Enabled() {
		if err := uc.tokenStore.Set(ctx, userID, refreshToken, refreshTokenTTL); err != nil {
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

	stored, err := uc.tokenStore.Get(ctx, claims.Subject)
	if err != nil || stored != cmd.RefreshToken {
		return nil, domain.NewError(domain.ErrUnauthorized, "Refresh token not valid or expired")
	}

	_, err = uc.userRepo.FindByID(ctx, claims.Subject)
	if err != nil {
		return nil, domain.NewError(domain.ErrUnauthorized, "User not found or deleted")
	}

	_ = uc.tokenStore.Delete(ctx, claims.Subject)

	newAccess, err := uc.tokenSigner.GenerateAccessToken(ctx, output.TokenClaims{
		Subject: claims.Subject,
	}, accessTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate new access token")
	}

	newRefresh, err := uc.tokenSigner.GenerateRefreshToken(ctx, claims.Subject, refreshTokenTTL)
	if err != nil {
		return nil, domain.NewError(domain.ErrInternalFailure, "Failed to generate new refresh token")
	}

	if err := uc.tokenStore.Set(ctx, claims.Subject, newRefresh, refreshTokenTTL); err != nil {
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

	return uc.tokenStore.Delete(ctx, claims.Subject)
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
