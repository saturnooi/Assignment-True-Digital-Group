package port

import (
	"context"
	"time"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value any, exp time.Duration) error
	Del(ctx context.Context, keys ...string) error
	DelPattern(ctx context.Context, pattern string) error
}
