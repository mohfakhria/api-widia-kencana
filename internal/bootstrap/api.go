package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"

	deliveryhttp "github.com/mohfakhria/api-widia-kencana/internal/delivery/http"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/database"
	pdfrender "github.com/mohfakhria/api-widia-kencana/internal/infrastructure/pdf"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/security"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/server"
	miniostorage "github.com/mohfakhria/api-widia-kencana/internal/infrastructure/storage/minio"
	memorystore "github.com/mohfakhria/api-widia-kencana/internal/persistence/memory"
	pg "github.com/mohfakhria/api-widia-kencana/internal/persistence/postgres"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/documentdesign"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"
)

type ApiApp struct {
	Context       context.Context
	ServiceLogger *slog.Logger
	Config        config.Config
	db            *sql.DB
	objectStorage output.ObjectStorage
	runner        *Service
	services      []ServiceStartup
}

func NewApiApp(ctx context.Context) *ApiApp {
	shared := NewShared()
	return &ApiApp{
		Context:       ctx,
		ServiceLogger: shared.Logger,
		Config:        shared.Config,
		runner:        NewService(shared.Logger),
	}
}

func (a *ApiApp) initialize() error {
	if err := a.Config.Validate(); err != nil {
		return err
	}

	db, err := database.NewPostgres(a.Context, a.Config)
	if err != nil {
		return err
	}
	a.db = db

	objectStorage, err := miniostorage.NewStorage(a.Context, a.Config)
	if err != nil {
		return err
	}
	a.objectStorage = objectStorage

	tokenSigner, err := security.NewJWTSigner(a.Config)
	if err != nil {
		return err
	}
	refreshTokenStore := memorystore.NewRefreshTokenStore()
	authUC := usecase.NewAuthUseCase(
		pg.NewUserRepository(a.db),
		refreshTokenStore,
		tokenSigner,
	)
	purchaseOrderUC := usecase.NewPurchaseOrderUseCase(
		pg.NewPurchaseOrderRepository(a.db),
		a.objectStorage,
	)
	assetUC := usecase.NewAssetUseCase(pg.NewAssetRepository(a.db), a.objectStorage)
	projectUC := usecase.NewProjectUseCase(pg.NewProjectRepository(a.db))
	documentRepo := pg.NewDocumentRepository(a.db)
	documentUC := usecase.NewDocumentUseCase(documentRepo)
	documentDesign := documentdesign.NewService(
		a.Context, documentRepo, deliveryhttp.DesignMessageEncoder{}, a.ServiceLogger,
	)

	// Font dimuat sekali saat start, bukan tiap ekspor. Kegagalan di sini
	// menghentikan aplikasi dengan sengaja: manifes yang cacat atau berkas yang
	// hilang berarti ekspor akan memakai huruf yang berbeda dari layar, dan itu
	// jauh lebih baik diketahui saat deploy daripada saat pengguna mencetak.
	fonts, err := pdfrender.LoadFonts(a.Config.DesignFontDir)
	if err != nil {
		return err
	}
	a.ServiceLogger.Info("loaded document export fonts",
		"dir", a.Config.DesignFontDir, "families", fonts.Families())

	documentExportUC := usecase.NewDocumentExportUseCase(
		documentRepo,
		documentDesign,
		pg.NewAssetRepository(a.db),
		a.objectStorage,
		pdfrender.NewRenderer(fonts),
	)
	quotationUC := usecase.NewQuotationUseCase(pg.NewQuotationRepository(a.db))
	workflowUC := usecase.NewWorkflowUseCase(pg.NewWorkflowRepository(a.db))
	workflowStageUC := usecase.NewWorkflowStageUseCase(pg.NewWorkflowStageRepository(a.db))
	workflowStepUC := usecase.NewWorkflowStepUseCase(pg.NewWorkflowStepRepository(a.db))

	router := deliveryhttp.NewRouter(deliveryhttp.RouterDeps{
		Config:          a.Config,
		TokenSigner:     tokenSigner,
		AssetHandler:    deliveryhttp.NewAssetHandler(assetUC),
		AuthHandler:     deliveryhttp.NewAuthHandler(authUC, a.Config),
		DocumentHandler: deliveryhttp.NewDocumentHandler(documentUC),
		DocumentDesignHandler: deliveryhttp.NewDocumentDesignHandler(
			a.Context, documentDesign, fonts, a.Config, a.ServiceLogger,
		),
		DocumentExportHandler: deliveryhttp.NewDocumentExportHandler(documentExportUC, a.ServiceLogger),
		ProjectHandler:        deliveryhttp.NewProjectHandler(projectUC),
		PurchaseOrderHandler:  deliveryhttp.NewPurchaseOrderHandler(purchaseOrderUC),
		QuotationHandler:      deliveryhttp.NewQuotationHandler(quotationUC),
		WorkflowHandler:       deliveryhttp.NewWorkflowHandler(workflowUC),
		WorkflowStageHandler:  deliveryhttp.NewWorkflowStageHandler(workflowStageUC),
		WorkflowStepHandler:   deliveryhttp.NewWorkflowStepHandler(workflowStepUC),
	})
	a.services = []ServiceStartup{
		server.NewHTTPServer(a.Config, router),
		refreshTokenStore,
		documentDesign,
	}

	return nil
}

func (a *ApiApp) Start() error {
	if err := a.initialize(); err != nil {
		return err
	}
	defer a.Cleanup()

	return a.runner.Run(a.Context, a.services)
}

func (a *ApiApp) Cleanup() {
	if a.db != nil {
		_ = a.db.Close()
	}
}
