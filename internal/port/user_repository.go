package port

import (
	"context"

	"github.com/saturnooi/recommendation-service/internal/domain/user"
)

type UserRepository interface {
	GetUserByID(ctx context.Context, id int64) (*user.User, error)
	GetUserWatchHistory(ctx context.Context, id int64) ([]user.WatchRecord, error)
	GetUnwatchedContent(ctx context.Context, id int64) ([]user.Content, error)
	GetUserIDsPaginated(ctx context.Context, limit, offset int) ([]int64, int, error)
	GetWatchHistoryByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]user.WatchRecord, error)
	GetTopContent(ctx context.Context) ([]user.Content, error)
}
