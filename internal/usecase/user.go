package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/saturnooi/recommendation-service/internal/domain"
	"github.com/saturnooi/recommendation-service/internal/errors/codes"
	"github.com/saturnooi/recommendation-service/internal/errors/httperr"
	"github.com/saturnooi/recommendation-service/internal/port"
)

type UserUsecase interface {
	GenerateRecommendations(ctx context.Context, id int64, limit int) (*GenerateRecommendationsResponse, error)
	RecordWatchHistory(ctx context.Context, userID, contentID int64) error
}

type userUsecase struct {
	repo  port.UserRepository
	model port.ModelClient
	cache port.Cache
}

func NewUserUsecase(r port.UserRepository, model port.ModelClient, cache port.Cache) UserUsecase {
	return &userUsecase{
		repo:  r,
		model: model,
		cache: cache,
	}
}

func (u *userUsecase) GenerateRecommendations(ctx context.Context, id int64, limit int) (*GenerateRecommendationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	cacheKey := fmt.Sprintf("rec:user:%d:limit:%d", id, limit)
	if val, err := u.cache.Get(ctx, cacheKey); err == nil {
		var cached GenerateRecommendationsResponse
		if err := json.Unmarshal([]byte(val), &cached); err == nil {
			cached.Metadata.CacheHit = true
			return &cached, nil
		}
	}

	userObj, err := u.repo.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, httperr.GatewayTimeout(
				codes.RequestTimeout,
				"Recommendation request timed out",
			)
		}
		return nil, err
	}
	if userObj == nil {
		return nil, httperr.NotFound(
			codes.UserNotFound,
			fmt.Sprintf("User with ID %d does not exist", id),
		)
	}

	history, err := u.repo.GetUserWatchHistory(ctx, id)
	if err != nil {
		return nil, err
	}
	pref := buildGenrePreference(history)

	candidates, err := u.repo.GetUnwatchedContent(ctx, id)
	if err != nil {
		return nil, err
	}

	scored, err := u.model.ScoreCandidates(*userObj, candidates, pref)
	if err != nil {
		return nil, err
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	recs := make([]Recommendation, 0, len(scored))
	for _, s := range scored {
		if ctx.Err() != nil {
			return nil, httperr.GatewayTimeout(
				codes.RequestTimeout,
				"Recommendation request timed out",
			)
		}

		recs = append(recs, Recommendation{
			ContentID:       s.Content.ID,
			Title:           s.Content.Title,
			Genre:           s.Content.Genre,
			PopularityScore: s.Content.PopularityScore,
			Score:           s.Score,
		})
	}

	response := GenerateRecommendationsResponse{
		UserID:          userObj.ID,
		Recommendations: recs,
		Metadata: Metadata{
			CacheHit:    false,
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			TotalCount:  len(recs),
		},
	}

	b, _ := json.Marshal(response)
	u.cache.Set(ctx, cacheKey, b, 10*time.Minute)

	return &response, nil
}

func (u *userUsecase) RecordWatchHistory(ctx context.Context, userID, contentID int64) error {
	if err := u.repo.RecordWatch(ctx, userID, contentID); err != nil {
		return err
	}
	pattern := fmt.Sprintf("rec:user:%d:limit:*", userID)
	return u.cache.DelPattern(ctx, pattern)
}

func buildGenrePreference(history []domain.WatchRecord) map[string]float64 {
	counts := map[string]int{}
	total := 0

	for _, h := range history {
		counts[h.Genre]++
		total++
	}

	if total == 0 {
		return map[string]float64{}
	}

	result := map[string]float64{}
	for k, v := range counts {
		result[k] = float64(v) / float64(total)
	}
	return result
}
