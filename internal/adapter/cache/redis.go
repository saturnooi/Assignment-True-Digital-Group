package cache

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
)

type ctxKeyClient struct{}

func New(ctx context.Context, url string, password string, db int) *redis.Client {
	rc := redis.NewClient(&redis.Options{
		Addr:     url,
		Password: password,
		DB:       db,
	})
	_, err := rc.Ping(ctx).Result()
	if err != nil {
		log.Fatalln("rdctx: cannot connect to Redis:", err.Error())
	}
	log.Println("rdctx: Redis connected")
	return rc
}

func NewWithContext(ctx context.Context, url string, password string, db int) (*redis.Client, context.Context) {
	rc := New(ctx, url, password, db)
	return rc, NewContext(ctx, rc)
}

func NewContext(ctx context.Context, rc *redis.Client) context.Context {
	return context.WithValue(ctx, ctxKeyClient{}, rc)
}

func Middleware(c *redis.Client) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(NewContext(r.Context(), c))
			h.ServeHTTP(w, r)
		})
	}
}

func c(ctx context.Context) *redis.Client {
	return ctx.Value(ctxKeyClient{}).(*redis.Client)
}

func Get(ctx context.Context, key string) (string, error) {
	return c(ctx).Get(ctx, key).Result()
}

func Set(ctx context.Context, key string, value interface{}, exp time.Duration) (string, error) {
	return c(ctx).Set(ctx, key, value, exp).Result()
}

func Del(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}
	return c(ctx).Del(ctx, keys...).Result()
}
