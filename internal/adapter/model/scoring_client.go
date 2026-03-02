package model

import (
	"math/rand"
	"time"

	"github.com/saturnooi/recommendation-service/internal/domain"
	"github.com/saturnooi/recommendation-service/internal/errors/codes"
	"github.com/saturnooi/recommendation-service/internal/errors/httperr"
	"github.com/saturnooi/recommendation-service/internal/port"
)

type ScoringClient struct{}

func NewScoringClient() port.ModelClient {
	return &ScoringClient{}
}

func (s *ScoringClient) ScoreCandidates(user domain.User, candidates []domain.Content, genrePreferences map[string]float64) ([]domain.ScoredContent, error) {
	delay := time.Duration(30+rand.Intn(20)) * time.Millisecond
	time.Sleep(delay)

	if rand.Float64() < 0.015 {
		return nil, httperr.ServiceUnavailable(
			codes.ModelUnavailable,
			"Recommendation model is temporarily unavailable",
		)
	}

	now := time.Now()

	scored := make([]domain.ScoredContent, 0, len(candidates))

	for _, content := range candidates {
		daysSinceCreation := now.Sub(content.CreatedAt).Hours() / 24
		recencyFactor := 1.0 / (1.0 + daysSinceCreation/365.0)

		popularityComponent := content.PopularityScore * 0.4
		genreBoost := genrePreferences[content.Genre] * 0.35
		recencyComponent := recencyFactor * 0.15
		randomNoise := (rand.Float64()*0.1 - 0.05) * 0.1

		finalScore :=
			popularityComponent +
				genreBoost +
				recencyComponent +
				randomNoise

		scored = append(scored, domain.ScoredContent{
			Content: content,
			Score:   finalScore,
		})
	}

	return scored, nil
}
