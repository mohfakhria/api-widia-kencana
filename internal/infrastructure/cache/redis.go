package cache

import (
	"context"

	"github.com/mohfakhria/api-widia-kencana/internal/infrastructure/config"

	"github.com/redis/go-redis/v9"
)

func NewRedis(ctx context.Context, cfg config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr(),
		DB:   0,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}
