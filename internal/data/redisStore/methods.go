package redisStore

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

func (s *Store) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return s.client.Set(ctx, key, value, expiration).Err()
}

func (s *Store) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s *Store) Del(ctx context.Context, keys ...string) error {
	return s.client.Del(ctx, keys...).Err()
}

func (s *Store) IsNil(err error) bool {
	return errors.Is(err, redis.Nil)
}

// this for the message store
func (s *Store) ListPush(ctx context.Context, key string, value interface{}) error {
	return s.client.RPush(ctx, key, value).Err()
}

func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	count, err := s.getCount(ctx, key)
	return count > 0, err
}

func (s *Store) getCount(ctx context.Context, key string) (int64, error) {
	return s.client.Exists(ctx, key).Result()
}

func (s *Store) ListGet5PastMessage(ctx context.Context, key string) ([]string, error) {
	count, err := s.getCount(ctx, key)
	if count < 1 || err != nil {
		return []string{}, err
	}
	if count < 5 {
		return s.ListGetAll(ctx, key)
	}
	return s.listGetPreviousXMessages(ctx, key, -5)
}

func (s *Store) ListGetAll(ctx context.Context, key string) ([]string, error) {
	return s.listGetPreviousXMessages(ctx, key, int64(0))
}

func (s *Store) listGetPreviousXMessages(ctx context.Context, key string, start int64) ([]string, error) {
	result, err := s.client.LRange(ctx, key, start, -1).Result()
	return result, err
}
