package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/mohfakhria/api-widia-kencana/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := bootstrap.NewApiApp(ctx)

	if err := app.Start(); err != nil {
		app.ServiceLogger.Error("ouchclient exited with error", "error", err)
		os.Exit(1)
	}

	app.ServiceLogger.Info("ouchclient shutdown complete")
}
