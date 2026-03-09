package cache

import (
	"context"
	"log"
	"runtime"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/saturnooi/recommendation-service/internal/port"
)

func New(ctx context.Context, url string, password string, db int) *redis.Client {
	rc := redis.NewClient(&redis.Options{
		Addr:         url,
		Password:     password,
		DB:           db,
		PoolSize:     10 * runtime.NumCPU(),
		MinIdleConns: 5,
	})
	_, err := rc.Ping(ctx).Result()
	if err != nil {
		log.Fatalln("rdctx: cannot connect to Redis:", err.Error())
	}
	log.Println("rdctx: Redis connected")
	return rc
}

type redisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) port.Cache {
	return &redisCache{client: client}
}

func (r *redisCache) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *redisCache) Set(ctx context.Context, key string, value any, exp time.Duration) error {
	return r.client.Set(ctx, key, value, exp).Err()
}

func (r *redisCache) Del(ctx context.Context, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	return r.client.Del(ctx, keys...).Err()
}

func (r *redisCache) DelPattern(ctx context.Context, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := r.client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
