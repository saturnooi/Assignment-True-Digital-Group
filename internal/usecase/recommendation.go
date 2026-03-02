package usecase

import (
	"context"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/saturnooi/recommendation-service/internal/domain"
	"github.com/saturnooi/recommendation-service/internal/port"
)

type RecommendationUsecase interface {
	GenerateBatch(ctx context.Context, page, limit int) (*BatchResponse, error)
}

type recommendationUsecase struct {
	repo  port.UserRepository
	model port.ModelClient
}

func NewRecommendationUsecase(r port.UserRepository, model port.ModelClient) RecommendationUsecase {
	return &recommendationUsecase{
		repo:  r,
		model: model,
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

	workers := min(limit, runtime.NumCPU()*2)

	jobs := make(chan int64)
	results := make(chan BatchResult, len(userIDs))

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				case userID, ok := <-jobs:
					if !ok {
						return
					}

					history := historyMap[userID]
					pref := buildGenrePreference(history)

					user := domain.User{ID: userID}

					scored, err := b.model.ScoreCandidates(
						user,
						candidates,
						pref,
					)

					if err != nil {
						results <- BatchResult{
							UserID:  userID,
							Status:  "failed",
							Error:   "model_unavailable",
							Message: err.Error(),
						}
						continue
					}

					sort.Slice(scored, func(i, j int) bool {
						return scored[i].Score > scored[j].Score
					})

					if len(scored) > 10 {
						scored = scored[:10]
					}

					recs := make([]Recommendation, 0, len(scored))
					for _, s := range scored {
						recs = append(recs, Recommendation{
							ContentID:       s.Content.ID,
							Title:           s.Content.Title,
							Genre:           s.Content.Genre,
							PopularityScore: s.Content.PopularityScore,
							Score:           s.Score,
						})
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
