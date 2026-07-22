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
	DocumentHandler      *DocumentHandler
	DocumentLayerHandler *DocumentLayerHandler
	ProjectHandler       *ProjectHandler
	PurchaseOrderHandler *PurchaseOrderHandler
	QuotationHandler     *QuotationHandler
	WorkflowHandler      *WorkflowHandler
	WorkflowStageHandler *WorkflowStageHandler
	WorkflowStepHandler  *WorkflowStepHandler
}

func NewRouter(deps RouterDeps) *gin.Engine {
	r := gin.Default()
	r.Use(corsMiddleware(deps.Config))

	r.GET("/health", func(c *gin.Context) {
		dto.Success(c, "ok", gin.H{"health": gin.H{"env": deps.Config.AppEnv}})
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
		protected.POST("/logout-all", deps.AuthHandler.LogoutAll)
		protected.GET("/document-master/papers", deps.DocumentHandler.ListPapers)
		protected.GET("/document-master/elements", deps.DocumentHandler.ListElements)
		protected.GET("/document-master/properties", deps.DocumentHandler.ListProperties)
		protected.GET("/document-master/property-options", deps.DocumentHandler.ListPropertyOptions)
		protected.GET("/document-master/element-properties", deps.DocumentHandler.ListElementProperties)
		protected.GET("/document-list", deps.DocumentHandler.List)
		protected.GET("/document-detail/:token", deps.DocumentHandler.Get)
		protected.POST("/document-add", deps.DocumentHandler.Create)
		protected.PUT("/document-update/:token", deps.DocumentHandler.Update)
		protected.DELETE("/document-delete/:token", deps.DocumentHandler.Delete)
		protected.POST("/document-layer-add", deps.DocumentLayerHandler.Create)
		protected.PUT("/document-layer-update/:token", deps.DocumentLayerHandler.Update)
		protected.PUT("/document-layer-sort", deps.DocumentLayerHandler.Sort)
		protected.DELETE("/document-layer-delete", deps.DocumentLayerHandler.Delete)
		protected.DELETE("/document-layer-delete/:token", deps.DocumentLayerHandler.Delete)
		protected.GET("/project-list", deps.ProjectHandler.List)
		protected.GET("/project-detail/:id", deps.ProjectHandler.Get)
		protected.POST("/project-add", deps.ProjectHandler.Create)
		protected.PUT("/project-update/:id", deps.ProjectHandler.Update)
		protected.DELETE("/project-delete/:id", deps.ProjectHandler.Delete)
		protected.GET("/workflow-list", deps.WorkflowHandler.List)
		protected.GET("/workflow-detail/:id", deps.WorkflowHandler.Get)
		protected.POST("/workflow-add", deps.WorkflowHandler.Create)
		protected.PUT("/workflow-update/:id", deps.WorkflowHandler.Update)
		protected.DELETE("/workflow-delete/:id", deps.WorkflowHandler.Delete)
		protected.GET("/workflow-stage-list/:workflowID", deps.WorkflowStageHandler.ListByWorkflowID)
		protected.GET("/workflow-stage-detail/:id", deps.WorkflowStageHandler.Get)
		protected.POST("/workflow-stage-add", deps.WorkflowStageHandler.Create)
		protected.PUT("/workflow-stage-update/:id", deps.WorkflowStageHandler.Update)
		protected.PUT("/workflow-stage-sort", deps.WorkflowStageHandler.Sort)
		protected.DELETE("/workflow-stage-delete/:id", deps.WorkflowStageHandler.Delete)
		protected.GET("/workflow-step-list/:workflowStageID", deps.WorkflowStepHandler.ListByWorkflowStageID)
		protected.GET("/workflow-step-detail/:id", deps.WorkflowStepHandler.Get)
		protected.POST("/workflow-step-add", deps.WorkflowStepHandler.Create)
		protected.PUT("/workflow-step-update/:id", deps.WorkflowStepHandler.Update)
		protected.PUT("/workflow-step-sort", deps.WorkflowStepHandler.Sort)
		protected.DELETE("/workflow-step-delete/:id", deps.WorkflowStepHandler.Delete)
		protected.POST("/purchase-order-upsert", deps.PurchaseOrderHandler.Upsert)
		protected.GET("/purchase-order/:quotationID", deps.PurchaseOrderHandler.GetByQuotationID)
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
