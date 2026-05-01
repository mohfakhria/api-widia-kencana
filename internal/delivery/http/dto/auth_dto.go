package dto

import "github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	UserID      string `json:"userid"`
	Name        string `json:"name"`
	Role        string `json:"role"`
}

type RefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
}

type ProfileResponse struct {
	UserID string `json:"userID"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

func NewLoginResponse(result *input.LoginResult) LoginResponse {
	return LoginResponse{
		AccessToken: result.AccessToken,
		UserID:      result.UserID,
		Name:        result.Name,
		Role:        result.Role,
	}
}

func NewRefreshTokenResponse(result *input.RefreshResult) RefreshTokenResponse {
	return RefreshTokenResponse{
		AccessToken: result.AccessToken,
	}
}

func NewProfileResponse(result *input.ProfileResult) ProfileResponse {
	return ProfileResponse{
		UserID: result.UserID,
		Name:   result.Name,
		Role:   result.Role,
	}
}
