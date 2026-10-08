package config

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisDB struct {
	db *redis.Client
}

func (r *RedisDB) String() string {
	return r.db.String()
}

func NewRedisDB(env *ENV) *RedisDB {
	return &RedisDB{
		db: redis.NewClient(&redis.Options{
			Addr:     env.Redis.Addr,
			Password: env.Redis.Password,
			DB:       env.Redis.DB,
			Protocol: env.Redis.Protocol,
		}),
	}
}

func (r *RedisDB) Close() error {
	return r.db.Close()
}

func (r *RedisDB) Get(ctx context.Context, key string) (string, error) {
	val, err := r.db.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}
	return val, nil
}

func (r *RedisDB) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return r.db.Set(ctx, key, value, ttl).Err()
}

func (r *RedisDB) SetNX(ctx context.Context, key string, value any, ttl time.Duration) (bool, error) {
	return r.db.SetNX(ctx, key, value, ttl).Result()
}

func (r *RedisDB) Exists(ctx context.Context, key string) (bool, error) {
	n, err := r.db.Exists(ctx, key).Result()
	return n > 0, err
}

func (r *RedisDB) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.db.Del(ctx, keys...).Err()
}

func (r *RedisDB) MGet(ctx context.Context, keys ...string) ([]*string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	vals, err := r.db.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]*string, len(vals))
	for i, v := range vals {
		if s, ok := v.(string); ok {
			out[i] = &s
		}
	}
	return out, nil
}

func (r *RedisDB) GetJSON(ctx context.Context, key string, out any) error {
	raw, err := r.db.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func (r *RedisDB) SetJSON(ctx context.Context, key string, ttl time.Duration, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return r.db.Set(ctx, key, raw, ttl).Err()
}

func (r *RedisDB) HSet(ctx context.Context, key string, value map[string]string, ttl time.Duration) error {
	if len(value) == 0 {
		return nil
	}
	if err := r.db.HSet(ctx, key, value).Err(); err != nil {
		return err
	}
	if ttl > 0 {
		return r.db.Expire(ctx, key, ttl).Err()
	}
	return nil
}

func (r *RedisDB) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	redisKey := fmt.Sprintf("feed:%s", key)
	raw, err := r.db.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, err
	}
	return raw, nil
}
