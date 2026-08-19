package http

import (
	"net/http"

	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/dto"
	"github.com/mohfakhria/api-widia-kencana/internal/delivery/http/middleware"
	"github.com/mohfakhria/api-widia-kencana/internal/domain/entity"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/gin-gonic/gin"
)

type RouterDeps struct {
	Config                config.Config
	TokenSigner           output.TokenSigner
	SessionStore          output.SessionStore
	AssetHandler          *AssetHandler
	FontHandler           *FontHandler
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

		// Alasan yang sama persis: rute ini dituju oleh aturan @font-face di
		// dalam CSS, yang juga tidak dapat mengirim header Authorization. Yang
		// dilayani berkas font berlisensi terbuka, bukan isi dokumen siapa pun.
		api.GET("/font-content/:family/:face", deps.FontHandler.Content)
	}

	// DUA grup, dan keduanya menyebut peran yang boleh masuk. Tidak ada grup
	// terautentikasi yang tanpa penjaga peran, dan itu disengaja: rute baru yang
	// ditambahkan ke protected — nama yang paling wajar diraih orang — otomatis
	// tertutup bagi agent. Kalau daftar izinnya ditempelkan per rute, rute yang
	// lupa dianotasi justru terbuka, dan lupa itu tidak menghasilkan galat apa
	// pun.
	//
	// Akibat lain yang layak diketahui: peran 'user' — bawaan kolom users.role,
	// yang tidak dimiliki satu baris pun — tertutup di mana-mana. Baris yang
	// disisipkan tanpa menyebut perannya lahir tanpa wewenang.

	// protected hanya untuk manusia.
	protected := r.Group("/api")
	protected.Use(
		middleware.AuthRequired(deps.TokenSigner, deps.SessionStore),
		middleware.RequireRole(entity.RoleSuperadmin),
	)
	{
		// Proyek adalah lapisan DI ATAS dokumen. Agent menyusun isi dokumen; ia
		// tidak menentukan dokumen mana yang ada dan milik siapa.
		protected.GET("/project-list", deps.ProjectHandler.List)
		protected.GET("/project-detail/:id", deps.ProjectHandler.Get)
		protected.POST("/project-add", deps.ProjectHandler.Create)
		protected.PUT("/project-update/:id", deps.ProjectHandler.Update)
		protected.DELETE("/project-delete/:id", deps.ProjectHandler.Delete)

		// Ekspor adalah tindakan manusia: agent menyusun, orang yang mencetak.
		protected.POST("/document-export/:token", deps.DocumentExportHandler.ExportPDF)

		// Font yang didaftarkan di sini disematkan ke SETIAP PDF sesudahnya. Ia
		// bukan bagian dari menyusun satu dokumen melainkan mengubah perkakas
		// yang dipakai semuanya, jadi ia berhenti di sini bersama proyek dan
		// ekspor.
		protected.POST("/font-add", deps.FontHandler.Register)
	}

	// agentAllowed dibuka untuk agent DI SAMPING manusia. Sengaja pendek, dan
	// menambah rute ke sini adalah tindakan sadar.
	agentAllowed := r.Group("/api")
	agentAllowed.Use(
		middleware.AuthRequired(deps.TokenSigner, deps.SessionStore),
		middleware.RequireRole(entity.RoleSuperadmin, entity.RoleAIAgent),
	)
	{
		agentAllowed.GET("/me", deps.AuthHandler.Me)
		agentAllowed.POST("/logout-all", deps.AuthHandler.LogoutAll)

		agentAllowed.POST("/asset-upload-request", deps.AssetHandler.RequestUpload)
		agentAllowed.POST("/asset-upload-complete/:token", deps.AssetHandler.CompleteUpload)
		agentAllowed.GET("/asset-list", deps.AssetHandler.List)
		agentAllowed.GET("/asset-detail/:token", deps.AssetHandler.Get)
		agentAllowed.GET("/asset-presign/:token", deps.AssetHandler.PresignGet)
		agentAllowed.DELETE("/asset-delete/:token", deps.AssetHandler.Delete)

		// Daftar font dibaca editor untuk menawarkan pilihan, jadi ia dibuka
		// selebar document-list. Yang dijaga superadmin adalah MENAMBAH font,
		// bukan mengetahui font apa yang ada.
		agentAllowed.GET("/font-list", deps.FontHandler.List)

		// Di grup yang sama dengan document-add, karena keduanya dipakai
		// berurutan: kertas dipilih lebih dulu, tokennya menjadi masukan wajib
		// bagi pembuatan dokumen.
		agentAllowed.GET("/paper-list", deps.DocumentHandler.ListPapers)

		agentAllowed.GET("/document-list", deps.DocumentHandler.List)
		agentAllowed.GET("/document-detail/:token", deps.DocumentHandler.Get)
		agentAllowed.POST("/document-add", deps.DocumentHandler.Create)
		agentAllowed.PUT("/document-update/:token", deps.DocumentHandler.Update)
		agentAllowed.DELETE("/document-delete/:token", deps.DocumentHandler.Delete)

		// Satu-satunya rute yang benar-benar WAJIB bagi agent: setelah tiket
		// terbit, seluruh penyuntingannya lewat socket, dan di sana tidak ada
		// barier sama sekali — ia penyunting penuh, termasuk undo dan redo.
		agentAllowed.POST("/document-design-ticket/:token", deps.DocumentDesignHandler.IssueTicket)
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
