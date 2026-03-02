package usecase

import (
	"context"
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/saturnooi/recommendation-service/internal/domain/user"
	"github.com/saturnooi/recommendation-service/internal/port"
)

type RecommendationUsecase interface {
	GenerateBatch(ctx context.Context, page, limit int) (*BatchResponse, error)
}

type recommendationUsecase struct {
	repo    port.UserRepository
	rng     *rand.Rand
	workers int
}

func NewRecommendationUsecase(r port.UserRepository) RecommendationUsecase {
	return &recommendationUsecase{
		repo:    r,
		workers: 8,
		rng:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

type BatchResult struct {
	UserID          int64                            `json:"user_id"`
	Status          string                           `json:"status"`
	Recommendations *GenerateRecommendationsResponse `json:"recommendations,omitempty"`
	Error           string                           `json:"error,omitempty"`
	Message         string                           `json:"message,omitempty"`
}

type BatchSummary struct {
	SuccessCount     int   `json:"success_count"`
	FailedCount      int   `json:"failed_count"`
	ProcessingTimeMs int64 `json:"processing_time_ms"`
}

type BatchResponse struct {
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalUsers int           `json:"total_users"`
	Results    []BatchResult `json:"results"`
	Summary    BatchSummary  `json:"summary"`
	Metadata   struct {
		GeneratedAt string `json:"generated_at"`
	} `json:"metadata"`
}

func (b *recommendationUsecase) GenerateBatch(ctx context.Context, page, limit int) (*BatchResponse, error) {
	start := time.Now()

	if page <= 0 {
		page = 1
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	offset := (page - 1) * limit

	b.workers = min(limit, runtime.NumCPU()*2)

	userIDs, totalUsers, err := b.repo.GetUserIDsPaginated(ctx, limit, offset)
	if err != nil {
		return nil, err
	}

	historyMap, err := b.repo.GetWatchHistoryByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}

	candidates, err := b.repo.GetTopContent(ctx)
	if err != nil {
		return nil, err
	}

	jobs := make(chan int64)
	results := make(chan BatchResult)

	var wg sync.WaitGroup

	for i := 0; i < b.workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			localRng := rand.New(rand.NewSource(time.Now().UnixNano()))

			for {
				select {
				case <-ctx.Done():
					return
				case userID, ok := <-jobs:
					if !ok {
						return
					}

					history := historyMap[userID]
					watchedSet := buildWatchedSet(history)
					pref := buildGenrePreference(history)

					var recs []Recommendation

					for _, c := range candidates {

						if _, ok := watchedSet[c.ID]; ok {
							continue
						}

						score := calculateScoreBatch(c, pref, localRng)

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

					if len(recs) > 10 {
						recs = recs[:10]
					}

					results <- BatchResult{
						UserID: userID,
						Status: "success",
						Recommendations: &GenerateRecommendationsResponse{
							UserID:          userID,
							Recommendations: recs,
						},
					}
				}
			}
		}()
	}

	go func() {
		for _, id := range userIDs {
			jobs <- id
		}
		close(jobs)
		wg.Wait()
		close(results)
	}()

	var batchResults []BatchResult
	success := 0
	failed := 0

	for r := range results {
		batchResults = append(batchResults, r)
		if r.Status == "success" {
			success++
		} else {
			failed++
		}
	}

	return &BatchResponse{
		Page:       page,
		Limit:      limit,
		TotalUsers: totalUsers,
		Results:    batchResults,
		Summary: BatchSummary{
			SuccessCount:     success,
			FailedCount:      failed,
			ProcessingTimeMs: time.Since(start).Milliseconds(),
		},
		Metadata: struct {
			GeneratedAt string `json:"generated_at"`
		}{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}

func buildWatchedSet(history []user.WatchRecord) map[int64]struct{} {
	set := make(map[int64]struct{})
	for _, h := range history {
		set[h.ContentID] = struct{}{}
	}
	return set
}

func calculateScoreBatch(c user.Content, pref map[string]float64, rng *rand.Rand) float64 {

	popularity := c.PopularityScore * 0.4

	genreWeight := 0.1
	if v, ok := pref[c.Genre]; ok {
		genreWeight = v
	}
	genreComponent := genreWeight * 0.35

	days := time.Since(c.CreatedAt).Hours() / 24
	recencyFactor := 1 / (1 + days/365)
	recencyComponent := recencyFactor * 0.15

	randomNoise := (rng.Float64()*0.1 - 0.05) * 0.1

	return popularity + genreComponent + recencyComponent + randomNoise
}
