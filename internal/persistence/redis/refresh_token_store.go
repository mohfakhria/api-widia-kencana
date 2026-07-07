package redis

import (
	"context"
	"encoding/json"
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

func (s *RefreshTokenStore) Set(ctx context.Context, sessionID string, session output.RefreshSession, ttl time.Duration) error {
	if !s.Enabled() {
		return nil
	}

	payload, err := json.Marshal(session)
	if err != nil {
		return err
	}

	pipe := s.client.TxPipeline()
	pipe.Set(ctx, sessionKey(sessionID), payload, ttl)
	pipe.SAdd(ctx, userSessionsKey(session.UserID), sessionID)
	pipe.Expire(ctx, userSessionsKey(session.UserID), ttl)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RefreshTokenStore) Get(ctx context.Context, sessionID string) (*output.RefreshSession, error) {
	payload, err := s.client.Get(ctx, sessionKey(sessionID)).Bytes()
	if err != nil {
		return nil, err
	}

	var session output.RefreshSession
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *RefreshTokenStore) Delete(ctx context.Context, userID, sessionID string) error {
	if !s.Enabled() {
		return nil
	}

	pipe := s.client.TxPipeline()
	pipe.Del(ctx, sessionKey(sessionID))
	pipe.SRem(ctx, userSessionsKey(userID), sessionID)
	_, err := pipe.Exec(ctx)
	return err

}

func (s *RefreshTokenStore) DeleteAll(ctx context.Context, userID string) error {
	if !s.Enabled() {
		return nil
	}

	sessionIDs, err := s.client.SMembers(ctx, userSessionsKey(userID)).Result()
	if err != nil {
		return err
	}

	keys := make([]string, 0, len(sessionIDs)+1)
	for _, sessionID := range sessionIDs {
		keys = append(keys, sessionKey(sessionID))
	}
	keys = append(keys, userSessionsKey(userID))
	return s.client.Del(ctx, keys...).Err()
}

func (s *RefreshTokenStore) Enabled() bool {
	return s.enabled
}

func sessionKey(sessionID string) string {
	return "refresh_session:" + sessionID
}

func userSessionsKey(userID string) string {
	return "user_refresh_sessions:" + userID
}
