package database

import (
	"context"
	"database/sql"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"

	_ "github.com/lib/pq"
)

func NewPostgres(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.PostgresDSN())
	if err != nil {
		return nil, err
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return db, nil
}
