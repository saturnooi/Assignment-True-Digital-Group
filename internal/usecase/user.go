package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/saturnooi/recommendation-service/internal/adapter/cache"
	"github.com/saturnooi/recommendation-service/internal/domain/user"
	"github.com/saturnooi/recommendation-service/internal/errors/codes"
	"github.com/saturnooi/recommendation-service/internal/errors/httperr"
	"github.com/saturnooi/recommendation-service/internal/port"
)

type UserUsecase interface {
	GenerateRecommendations(ctx context.Context, id int64, limit int) (*GenerateRecommendationsResponse, error)
}

type userUsecase struct {
	repo port.UserRepository
	rng  *rand.Rand
}

func NewUserUsecase(r port.UserRepository) UserUsecase {
	return &userUsecase{
		repo: r,
		rng:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type Recommendation struct {
	ContentID       int64   `json:"content_id"`
	Title           string  `json:"title"`
	Genre           string  `json:"genre"`
	PopularityScore float64 `json:"popularity_score"`
	Score           float64 `json:"score"`
}

type Metadata struct {
	CacheHit    bool   `json:"cache_hit"`
	GeneratedAt string `json:"generated_at"`
	TotalCount  int    `json:"total_count"`
}

type GenerateRecommendationsResponse struct {
	UserID          int64            `json:"user_id"`
	Recommendations []Recommendation `json:"recommendations"`
	Metadata        Metadata         `json:"metadata"`
}

func (u *userUsecase) GenerateRecommendations(ctx context.Context, id int64, limit int) (*GenerateRecommendationsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()

	if limit <= 0 || limit > 50 {
		limit = 10
	}

	cacheKey := fmt.Sprintf("recommendation:%d:%d", id, limit)
	if val, err := cache.Get(ctx, cacheKey); err == nil {
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

	delay := time.Duration(30+u.rng.Intn(21)) * time.Millisecond

	select {
	case <-time.After(delay):
	case <-ctx.Done():
		return nil, httperr.GatewayTimeout(
			codes.RequestTimeout,
			"Recommendation request timed out",
		)
	}

	if u.rng.Float64() < 0.015 {
		return nil, httperr.ServiceUnavailable(
			codes.ModelUnavailable,
			"Recommendation model is temporarily unavailable",
		)
	}

	var recs []Recommendation
	for _, c := range candidates {
		if ctx.Err() != nil {
			return nil, httperr.GatewayTimeout(
				codes.RequestTimeout,
				"Recommendation request timed out",
			)
		}

		score := u.calculateScore(c, pref)
		recs = append(recs, Recommendation{
			ContentID:       c.ID,
			Title:           c.Title,
			Genre:           c.Genre,
			PopularityScore: c.PopularityScore,
			Score:           score,
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Score > recs[j].Score
	})

	if len(recs) > limit {
		recs = recs[:limit]
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

	cache.Set(ctx, cacheKey, b, 10*time.Minute)
	return &response, nil
}

func buildGenrePreference(history []user.WatchRecord) map[string]float64 {
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

func (u *userUsecase) calculateScore(c user.Content, pref map[string]float64) float64 {

	popularity := c.PopularityScore * 0.4

	genreWeight := 0.1
	if v, ok := pref[c.Genre]; ok {
		genreWeight = v
	}
	genreComponent := genreWeight * 0.35

	days := time.Since(c.CreatedAt).Hours() / 24
	recencyFactor := 1 / (1 + days/365)
	recencyComponent := recencyFactor * 0.15

	randomNoise := (u.rng.Float64()*0.1 - 0.05) * 0.1

	return popularity + genreComponent + recencyComponent + randomNoise
}
