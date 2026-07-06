package dto

import "github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	ExpiredAt   int64  `json:"expired_at"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiredAt   int64  `json:"expired_at"`
}

type ProfileResponse struct {
	UserID string `json:"userID"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

func NewLoginResponse(result *input.LoginResult) LoginResponse {
	return LoginResponse{
		AccessToken: result.AccessToken,
		ExpiredAt:   result.AccessExpiredAt,
	}
}

func NewRefreshTokenResponse(result *input.RefreshResult) RefreshTokenResponse {
	return RefreshTokenResponse{
		AccessToken: result.AccessToken,
		ExpiredAt:   result.AccessExpiredAt,
	}
}

func NewProfileResponse(result *input.ProfileResult) ProfileResponse {
	return ProfileResponse{
		UserID: result.UserID,
		Name:   result.Name,
		Role:   result.Role,
	}
}
