package input

import (
	"context"
	"time"
)

type AuthUseCase interface {
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)
	RefreshToken(ctx context.Context, cmd RefreshCommand) (*RefreshResult, error)
	Logout(ctx context.Context, cmd LogoutCommand) error
	GetProfile(ctx context.Context, cmd GetProfileCommand) (*ProfileResult, error)
}

type LoginCommand struct {
	Email    string
	Password string
}

type LoginResult struct {
	AccessToken     string
	AccessExpiredAt int64
	RefreshToken    string
	RefreshTokenTTL time.Duration
}

type RefreshCommand struct {
	RefreshToken string
}

type RefreshResult struct {
	AccessToken     string
	RefreshToken    string
	RefreshTokenTTL time.Duration
}

type LogoutCommand struct {
	RefreshToken string
}

type GetProfileCommand struct {
	UserID string
}

type ProfileResult struct {
	UserID string
	Name   string
	Role   string
}
