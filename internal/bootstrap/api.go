package bootstrap

import (
	"context"
	"database/sql"
	"log/slog"

	deliveryhttp "github.com/mohfakhria/api-widia-kencana/internal/delivery/http"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/cache"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/database"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/security"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/server"
	pg "github.com/mohfakhria/api-widia-kencana/internal/persistence/postgres"
	redisstore "github.com/mohfakhria/api-widia-kencana/internal/persistence/redis"
	"github.com/mohfakhria/api-widia-kencana/internal/usecase"

	"github.com/redis/go-redis/v9"
)

type ApiApp struct {
	Context       context.Context
	ServiceLogger *slog.Logger
	Config        config.Config
	db            *sql.DB
	redisClient   *redis.Client
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
	db, err := database.NewPostgres(a.Context, a.Config)
	if err != nil {
		return err
	}
	a.db = db

	if a.Config.RedisEnabled {
		client, err := cache.NewRedis(a.Context, a.Config)
		if err != nil {
			return err
		}
		a.redisClient = client
	}

	tokenSigner := security.NewJWTSigner(a.Config)
	authUC := usecase.NewAuthUseCase(
		pg.NewUserRepository(a.db),
		redisstore.NewRefreshTokenStore(a.redisClient, a.Config.RedisEnabled),
		tokenSigner,
	)
	purchaseOrderUC := usecase.NewPurchaseOrderUseCase(pg.NewPurchaseOrderRepository(a.db))
	quotationUC := usecase.NewQuotationUseCase(pg.NewQuotationRepository(a.db))

	router := deliveryhttp.NewRouter(deliveryhttp.RouterDeps{
		Config:               a.Config,
		TokenSigner:          tokenSigner,
		AuthHandler:          deliveryhttp.NewAuthHandler(authUC, a.Config),
		PurchaseOrderHandler: deliveryhttp.NewPurchaseOrderHandler(purchaseOrderUC),
		QuotationHandler:     deliveryhttp.NewQuotationHandler(quotationUC),
	})
	a.services = []ServiceStartup{
		server.NewHTTPServer(a.Config, router),
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
	if a.redisClient != nil {
		_ = a.redisClient.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}
