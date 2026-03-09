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

func (b *recommendationUsecase) GenerateBatch(ctx context.Context, page, limit int) (*BatchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

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

	// Bulk-fetch user profiles, watch histories, and top content to avoid N+1 queries.
	usersMap, err := b.repo.GetUsersByIDs(ctx, userIDs)
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
	resultsCh := make(chan BatchResult, len(userIDs))

	var wg sync.WaitGroup

	for range workers {
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

					user := usersMap[userID]
					history := historyMap[userID]
					pref := buildGenrePreference(history)

					// Filter out content already watched by this user.
					watchedIDs := make(map[int64]struct{}, len(history))
					for _, h := range history {
						watchedIDs[h.ContentID] = struct{}{}
					}
					filtered := make([]domain.Content, 0, len(candidates))
					for _, c := range candidates {
						if _, watched := watchedIDs[c.ID]; !watched {
							filtered = append(filtered, c)
						}
					}

					scored, err := b.model.ScoreCandidates(user, filtered, pref)
					if err != nil {
						resultsCh <- BatchResult{
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

					resultsCh <- BatchResult{
						UserID:          userID,
						Status:          "success",
						Recommendations: recs,
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
		close(resultsCh)
	}()

	var batchResults []BatchResult
	success := 0
	failed := 0

	for r := range resultsCh {
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
		Metadata: BatchMetadata{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}, nil
}
