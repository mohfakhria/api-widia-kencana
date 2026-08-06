package bootstrap

import (
	"log/slog"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"
	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/logger"
)

type Shared struct {
	Config config.Config
	Logger *slog.Logger
}

func NewShared() *Shared {
	cfg := config.Load()

	return &Shared{
		Config: cfg,
		Logger: logger.New(cfg.LogLevel),
	}
}
