package usecase

// Recommendation is a single content item with its computed score.
type Recommendation struct {
	ContentID       int64   `json:"content_id"`
	Title           string  `json:"title"`
	Genre           string  `json:"genre"`
	PopularityScore float64 `json:"popularity_score"`
	Score           float64 `json:"score"`
}

// Metadata carries response-level information for the single-user endpoint.
type Metadata struct {
	CacheHit    bool   `json:"cache_hit"`
	GeneratedAt string `json:"generated_at"`
	TotalCount  int    `json:"total_count"`
}

// GenerateRecommendationsResponse is the response for the single-user endpoint.
type GenerateRecommendationsResponse struct {
	UserID          int64            `json:"user_id"`
	Recommendations []Recommendation `json:"recommendations"`
	Metadata        Metadata         `json:"metadata"`
}

// BatchResult holds the outcome (success or failure) for one user in a batch.
type BatchResult struct {
	UserID          int64            `json:"user_id"`
	Status          string           `json:"status"`
	Recommendations []Recommendation `json:"recommendations,omitempty"`
	Error           string           `json:"error,omitempty"`
	Message         string           `json:"message,omitempty"`
}

// BatchSummary aggregates success/failure counts and elapsed time for a batch.
type BatchSummary struct {
	SuccessCount     int   `json:"success_count"`
	FailedCount      int   `json:"failed_count"`
	ProcessingTimeMs int64 `json:"processing_time_ms"`
}

// BatchMetadata carries response-level information for the batch endpoint.
type BatchMetadata struct {
	GeneratedAt string `json:"generated_at"`
}

// BatchResponse is the response for the batch endpoint.
type BatchResponse struct {
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalUsers int           `json:"total_users"`
	Results    []BatchResult `json:"results"`
	Summary    BatchSummary  `json:"summary"`
	Metadata   BatchMetadata `json:"metadata"`
}
