package port

import (
	"context"

	"github.com/saturnooi/recommendation-service/internal/domain"
)

type UserRepository interface {
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
	GetUserWatchHistory(ctx context.Context, id int64) ([]domain.WatchRecord, error)
	GetUnwatchedContent(ctx context.Context, id int64) ([]domain.Content, error)
	GetUserIDsPaginated(ctx context.Context, limit, offset int) ([]int64, int, error)
	GetWatchHistoryByUserIDs(ctx context.Context, domainIDs []int64) (map[int64][]domain.WatchRecord, error)
	GetTopContent(ctx context.Context) ([]domain.Content, error)
}
