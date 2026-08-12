package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mohfakhria/api-widia-kencana/internal/mcp"

	"github.com/joho/godotenv"
)

func main() {
	// Berkasnya SENDIRI, bukan .env milik API.
	//
	// .env API memuat sandi database, kredensial MinIO, dan JWT_SECRET — tidak
	// satu pun urusan MCP. Memuatnya berarti seluruh rahasia itu masuk ke
	// environment proses ini tanpa alasan, dan proses yang memegang rahasia yang
	// tidak ia butuhkan hanya menambah tempat rahasia itu dapat bocor.
	//
	// Tidak ada berkasnya juga bukan galat: di server, env datang dari unit
	// systemd.
	_ = godotenv.Load(".env.mcp")

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := mcp.LoadConfig()
	if err != nil {
		// Berhenti sebelum mendengarkan, bukan sesudah. Server yang menyala tanpa
		// kredensial atau tanpa token penjaga jauh lebih buruk daripada server
		// yang menolak berdiri.
		logger.Error("konfigurasi mcp tidak lengkap", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	server := mcp.NewServer(cfg, mcp.NewAPIClient(cfg, logger), logger)
	if err := server.Run(ctx); err != nil {
		logger.Error("mcp berhenti dengan galat", "error", err)
		os.Exit(1)
	}

	logger.Info("mcp shutdown complete")
}
