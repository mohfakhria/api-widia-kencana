package http

import (
	"errors"
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	auth input.AuthUseCase
	cfg  config.Config
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func NewAuthHandler(auth input.AuthUseCase, cfg config.Config) *AuthHandler {
	return &AuthHandler{auth: auth, cfg: cfg}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	result, err := h.auth.Login(c.Request.Context(), input.LoginCommand{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	if h.cfg.RedisEnabled {
		c.SetSameSite(http.SameSiteStrictMode)
		c.SetCookie("refresh_token", result.RefreshToken, int(result.RefreshTokenTTL.Seconds()), "/", h.cfg.CookieDomain(), h.cfg.CookieSecure(), true)
	}

	Success(c, "Login success", gin.H{
		"access_token": result.AccessToken,
		"userid":       result.UserID,
		"name":         result.Name,
		"role":         result.Role,
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	cookie, err := c.Cookie("refresh_token")
	if err != nil {
		Error(c, http.StatusUnauthorized, "Missing refresh token")
		return
	}

	result, err := h.auth.RefreshToken(c.Request.Context(), input.RefreshCommand{
		RefreshToken: cookie,
	})
	if err != nil {
		Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie("refresh_token", result.RefreshToken, int(result.RefreshTokenTTL.Seconds()), "/", h.cfg.CookieDomain(), h.cfg.CookieSecure(), true)
	Success(c, "Token refreshed successfully", gin.H{
		"access_token": result.AccessToken,
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	cookie, _ := c.Cookie("refresh_token")
	_ = h.auth.Logout(c.Request.Context(), input.LogoutCommand{
		RefreshToken: cookie,
	})

	c.SetCookie("refresh_token", "", -1, "/", h.cfg.CookieDomain(), h.cfg.CookieSecure(), true)
	Success(c, "Logout successful", nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	result, err := h.auth.GetProfile(c.Request.Context(), input.GetProfileCommand{
		UserID: c.GetString("userID"),
		Name:   c.GetString("name"),
		Role:   c.GetString("role"),
	})
	if err != nil {
		Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	Success(c, "User profile", gin.H{
		"userID": result.UserID,
		"name":   result.Name,
		"role":   result.Role,
	})
}

func UnauthorizedMessage(err error) string {
	if errors.Is(err, domain.ErrUnauthorized) {
		return err.Error()
	}
	return "Invalid or expired token"
}
