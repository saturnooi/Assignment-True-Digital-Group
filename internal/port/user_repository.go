package port

import (
	"context"

	"github.com/saturnooi/recommendation-service/internal/domain/user"
)

type UserRepository interface {
	GetUserByID(ctx context.Context, id int64) (*user.User, error)
	GetUserWatchHistory(ctx context.Context, id int64) ([]user.WatchRecord, error)
	GetUnwatchedContent(ctx context.Context, id int64) ([]user.Content, error)
}
