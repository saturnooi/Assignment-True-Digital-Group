package port

import "github.com/saturnooi/recommendation-service/internal/domain"

type ModelClient interface {
	ScoreCandidates(user domain.User, candidates []domain.Content, genrePreferences map[string]float64) ([]domain.ScoredContent, error)
}
