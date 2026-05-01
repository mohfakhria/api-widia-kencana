package redis

import (
	"context"
	"time"

	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	redislib "github.com/redis/go-redis/v9"
)

type RefreshTokenStore struct {
	client  *redislib.Client
	enabled bool
}

func NewRefreshTokenStore(client *redislib.Client, enabled bool) output.RefreshTokenStore {
	return &RefreshTokenStore{
		client:  client,
		enabled: enabled && client != nil,
	}
}

func (s *RefreshTokenStore) Set(ctx context.Context, userID, token string, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}
	return s.client.Set(ctx, key(userID), token, ttl).Err()
}

func (s *RefreshTokenStore) Get(ctx context.Context, userID string) (string, error) {
	return s.client.Get(ctx, key(userID)).Result()
}

func (s *RefreshTokenStore) Delete(ctx context.Context, userID string) error {
	if !s.Enabled() {
		return nil
	}
	return s.client.Del(ctx, key(userID)).Err()
}

func (s *RefreshTokenStore) Enabled() bool {
	return s.enabled
}

func key(userID string) string {
	return "refresh_token:" + userID
}
