package config

import (
	"context"
	"fmt"

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

func (r *RedisDB) Set(ctx context.Context, key string, value string) error {
	return r.db.Set(ctx, key, value, 0).Err()
}
func (r *RedisDB) HSet(ctx context.Context, key string, value map[string]string) error {
	if len(value) == 0 {
		return nil // nothing to set
	}
	return r.db.HSet(ctx, key, value).Err()
}
func (r *RedisDB) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	redisKey := fmt.Sprintf("feed:%s", key)
	raw, err := r.db.HGetAll(ctx, redisKey).Result()
	if err != nil {
		return nil, err
	}

	// Convert map[string]string → map[string]models.FeedType
	// feedLinks := make(map[string]models.FeedType, len(raw))
	// for k, v := range raw {
	// 	feedLinks[k] = models.FeedType(v)
	// }
	return raw, nil
}
