package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/middleware"
	"github.com/mohfakhria/api-widia-kencana/internal/domain"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/input"
	"github.com/mohfakhria/api-widia-kencana/pkg/apperror"

	"github.com/gin-gonic/gin"
)

const (
	refreshCookieName = "refresh_token"
	refreshCookiePath = "/"
)

type AuthHandler struct {
	auth input.AuthUseCase
	cfg  config.Config
}

func NewAuthHandler(auth input.AuthUseCase, cfg config.Config) *AuthHandler {
	return &AuthHandler{auth: auth, cfg: cfg}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, http.StatusBadRequest, "Invalid request payload")
		return
	}

	result, err := h.auth.Login(c.Request.Context(), input.LoginCommand{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	h.setRefreshCookie(c, result.RefreshToken, result.RefreshTokenTTL)
	dto.Success(c, "Login success", dto.NewLoginResponse(result))
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	cookie, err := c.Cookie(refreshCookieName)
	if err != nil {
		dto.Error(c, http.StatusUnauthorized, "Missing refresh token")
		return
	}

	result, err := h.auth.RefreshToken(c.Request.Context(), input.RefreshCommand{
		RefreshToken: cookie,
	})
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	h.setRefreshCookie(c, result.RefreshToken, result.RefreshTokenTTL)
	dto.Success(c, "Token refreshed successfully", dto.NewRefreshTokenResponse(result))
}

func (h *AuthHandler) Logout(c *gin.Context) {
	cookie, _ := c.Cookie(refreshCookieName)
	if err := h.auth.Logout(c.Request.Context(), input.LogoutCommand{
		RefreshToken: cookie,
	}); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	h.clearRefreshCookie(c)
	dto.Success(c, "Logout successful", nil)
}

func (h *AuthHandler) LogoutAll(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	if err := h.auth.LogoutAll(c.Request.Context(), input.LogoutAllCommand{
		UserID: user.UserID,
	}); err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	h.clearRefreshCookie(c)
	dto.Success(c, "Logged out from all devices successfully", nil)
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, _ := middleware.CurrentUser(c)
	result, err := h.auth.GetProfile(c.Request.Context(), input.GetProfileCommand{
		UserID: user.UserID,
	})
	if err != nil {
		dto.Error(c, apperror.ToHTTPStatus(err), err.Error())
		return
	}

	dto.Success(c, "User profile", dto.NewProfileResponse(result))
}

// setRefreshCookie dan clearRefreshCookie memusatkan atribut keamanan cookie
// refresh token, supaya penulisan dan penghapusannya tidak bisa menyimpang.
// Cookie dihapus browser hanya bila Name, Path, dan Domain-nya sama persis.
func (h *AuthHandler) setRefreshCookie(c *gin.Context, token string, ttl time.Duration) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, token, int(ttl.Seconds()), refreshCookiePath, h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

func (h *AuthHandler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, refreshCookiePath, h.cfg.CookieDomain, h.cfg.CookieSecure, true)
}

func UnauthorizedMessage(err error) string {
	if errors.Is(err, domain.ErrUnauthorized) {
		return err.Error()
	}
	return "Invalid or expired token"
}
