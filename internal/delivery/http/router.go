package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/middleware"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/gin-gonic/gin"
)

type RouterDeps struct {
	Config                config.Config
	TokenSigner           output.TokenSigner
	SessionStore          output.SessionStore
	AssetHandler          *AssetHandler
	AuthHandler           *AuthHandler
	DocumentHandler       *DocumentHandler
	DocumentDesignHandler *DocumentDesignHandler
	DocumentExportHandler *DocumentExportHandler
	ProjectHandler        *ProjectHandler
}

func NewRouter(deps RouterDeps) http.Handler {
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

		// Di grup publik, dan itu keharusan bukan kelalaian: rute ini dituju
		// langsung oleh tag <img>, yang tidak dapat mengirim header Authorization.
		// Token aset yang menjadi kredensialnya — lihat AssetHandler.Content.
		api.GET("/asset-content/:token", deps.AssetHandler.Content)
	}

	protected := r.Group("/api")
	protected.Use(middleware.AuthRequired(deps.TokenSigner, deps.SessionStore))
	{
		protected.GET("/me", deps.AuthHandler.Me)
		protected.POST("/logout-all", deps.AuthHandler.LogoutAll)
		protected.POST("/asset-upload-request", deps.AssetHandler.RequestUpload)
		protected.POST("/asset-upload-complete/:token", deps.AssetHandler.CompleteUpload)
		protected.GET("/asset-list", deps.AssetHandler.List)
		protected.GET("/asset-detail/:token", deps.AssetHandler.Get)
		protected.GET("/asset-presign/:token", deps.AssetHandler.PresignGet)
		protected.DELETE("/asset-delete/:token", deps.AssetHandler.Delete)
		protected.GET("/document-list", deps.DocumentHandler.List)
		protected.GET("/document-detail/:token", deps.DocumentHandler.Get)
		protected.POST("/document-add", deps.DocumentHandler.Create)
		protected.PUT("/document-update/:token", deps.DocumentHandler.Update)
		protected.DELETE("/document-delete/:token", deps.DocumentHandler.Delete)
		protected.POST("/document-design-ticket/:token", deps.DocumentDesignHandler.IssueTicket)
		protected.POST("/document-export/:token", deps.DocumentExportHandler.ExportPDF)
		protected.GET("/project-list", deps.ProjectHandler.List)
		protected.GET("/project-detail/:id", deps.ProjectHandler.Get)
		protected.POST("/project-add", deps.ProjectHandler.Create)
		protected.PUT("/project-update/:id", deps.ProjectHandler.Update)
		protected.DELETE("/project-delete/:id", deps.ProjectHandler.Delete)
	}

	// Handshake WebSocket dilayani di luar gin. Upgrade harus mengambil alih
	// koneksi mentah, sedangkan gin v1.11 menolak hijack begitu responsnya
	// tersentuh. Selain itu handshake tidak melewati AuthRequired karena browser
	// tidak bisa mengirim header Authorization; pengamanannya berupa tiket
	// sekali pakai dari endpoint terproteksi di atas, plus pemeriksaan Origin.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /document-design/{token}", deps.DocumentDesignHandler.Connect)
	mux.Handle("/", r)

	return mux
}

func corsMiddleware(cfg config.Config) gin.HandlerFunc {
	patterns := allowedOriginPatterns(cfg)

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Vary WAJIB ada begitu jawabannya bergantung pada Origin. Tanpa itu,
		// cache di depan API dapat menyimpan jawaban beserta header
		// Allow-Origin milik satu situs, lalu menyajikannya kepada situs lain.
		c.Writer.Header().Add("Vary", "Origin")

		if originAllowed(patterns, origin) {
			// Yang dipantulkan origin ASLI dari permintaan, bukan polanya.
			// Access-Control-Allow-Origin tidak mengenal glob, dan browser
			// membandingkannya persis dengan origin miliknya sendiri.
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
			c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
			c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
