package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	Client *redis.Client
}

func NewRedisStore(opt *redis.Options) *RedisStore {
	return &RedisStore{Client: redis.NewClient(opt)}
}

func (s *RedisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	b, err := s.Client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.Client.Set(ctx, key, value, ttl).Err()
}

func (s *RedisStore) Delete(ctx context.Context, key string) error {
	return s.Client.Del(ctx, key).Err()
}
