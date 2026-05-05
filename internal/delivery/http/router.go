package http

import (
	"os"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/middleware"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/gin-gonic/gin"
)

type RouterDeps struct {
	Config               config.Config
	TokenSigner          output.TokenSigner
	AuthHandler          *AuthHandler
	PurchaseOrderHandler *PurchaseOrderHandler
	QuotationHandler     *QuotationHandler
}

func NewRouter(deps RouterDeps) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware(deps.Config))

	r.GET("/health", func(c *gin.Context) {
		dto.Success(c, "ok", gin.H{"env": deps.Config.AppEnv})
	})

	api := r.Group("/api")
	{
		api.POST("/login", deps.AuthHandler.Login)
		api.POST("/refresh-token", deps.AuthHandler.RefreshToken)
		api.POST("/logout", deps.AuthHandler.Logout)
	}

	protected := r.Group("/api")
	protected.Use(middleware.AuthRequired(deps.TokenSigner))
	{
		protected.GET("/me", deps.AuthHandler.Me)
		protected.POST("/purchase-order-upsert", deps.PurchaseOrderHandler.Upsert)
		protected.GET("/purchase-order/:quotationID", deps.PurchaseOrderHandler.GetByQuotationID)
		protected.DELETE("/purchase-order/:quotationID", deps.PurchaseOrderHandler.DeleteByQuotationID)
		protected.GET("/quotation-list", deps.QuotationHandler.List)
		protected.GET("/quotation-detail/:id", deps.QuotationHandler.Get)
		protected.PUT("/quotation-update/:id", deps.QuotationHandler.Update)
		protected.POST("/quotation-add", deps.QuotationHandler.Create)
	}

	return r
}

func corsMiddleware(cfg config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowedOrigin := os.Getenv("FRONTEND_URL")
		if allowedOrigin == "" {
			allowedOrigin = cfg.FrontendURL
		}

		if origin == allowedOrigin {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
