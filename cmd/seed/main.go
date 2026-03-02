package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	_ "github.com/lib/pq"
	"github.com/saturnooi/recommendation-service/cmd/seed/config"
)

const (
	userCount         = 2000
	contentCount      = 5000
	watchHistoryCount = 20000
)

type ContentItem struct {
	ID         int
	Popularity float64
}

func main() {
	r := rand.New(rand.NewSource(42))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c := config.Init()

	db, err := sql.Open("postgres", c.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		log.Fatal(err)
	}

	if err := truncateTables(ctx, db); err != nil {
		log.Fatal(err)
	}

	users := seedUsers(ctx, db, r, userCount)
	content := seedContent(ctx, db, r, contentCount)
	seedWatchHistory(ctx, db, r, users, content, watchHistoryCount)

	log.Println("Seeding completed successfully")
}

func truncateTables(ctx context.Context, db *sql.DB) error {
	queries := []string{
		"TRUNCATE user_watch_history RESTART IDENTITY CASCADE",
		"TRUNCATE content RESTART IDENTITY CASCADE",
		"TRUNCATE users RESTART IDENTITY CASCADE",
	}

	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func seedUsers(ctx context.Context, db *sql.DB, r *rand.Rand, n int) []int {
	tx, _ := db.BeginTx(ctx, nil)
	defer tx.Rollback()

	stmt, _ := tx.PrepareContext(ctx,
		"INSERT INTO users(age, country, subscription_type) VALUES($1,$2,$3) RETURNING id")
	defer stmt.Close()

	countries := []string{"US", "GB", "CA", "AU", "DE", "TH", "JP", "FR", "SG", "KR"}
	subscriptions := []string{"free", "basic", "premium"}
	subWeights := []float64{0.5, 0.3, 0.2}

	ids := make([]int, 0, n)

	for i := 0; i < n; i++ {
		age := r.Intn(47) + 18
		country := countries[r.Intn(len(countries))]
		sub := weightedChoice(r, subscriptions, subWeights)

		var id int
		_ = stmt.QueryRowContext(ctx, age, country, sub).Scan(&id)
		ids = append(ids, id)
	}

	_ = tx.Commit()
	return ids
}

func seedContent(ctx context.Context, db *sql.DB, r *rand.Rand, n int) []ContentItem {
	tx, _ := db.BeginTx(ctx, nil)
	defer tx.Rollback()

	stmt, _ := tx.PrepareContext(ctx,
		"INSERT INTO content(title, genre, popularity_score, created_at) VALUES($1,$2,$3,$4) RETURNING id")
	defer stmt.Close()

	genres := []string{
		"action", "drama", "comedy", "thriller", "documentary",
		"sci-fi", "romance", "horror", "fantasy", "animation",
	}

	items := make([]ContentItem, 0, n)

	for i := 0; i < n; i++ {
		title := fmt.Sprintf("Movie %d", i+1)
		genre := genres[r.Intn(len(genres))]

		popularity := math.Pow(r.Float64(), 2)

		createdAt := time.Now().AddDate(0, 0, -r.Intn(365))

		var id int
		_ = stmt.QueryRowContext(ctx, title, genre, popularity, createdAt).Scan(&id)

		items = append(items, ContentItem{
			ID:         id,
			Popularity: popularity,
		})
	}

	_ = tx.Commit()
	return items
}

func seedWatchHistory(ctx context.Context, db *sql.DB, r *rand.Rand, users []int, content []ContentItem, n int) {
	tx, _ := db.BeginTx(ctx, nil)
	defer tx.Rollback()

	stmt, _ := tx.PrepareContext(ctx,
		"INSERT INTO user_watch_history(user_id, content_id, watched_at) VALUES($1,$2,$3)")
	defer stmt.Close()

	totalWeight := 0.0
	for _, c := range content {
		totalWeight += c.Popularity
	}

	for i := 0; i < n; i++ {
		userID := users[r.Intn(len(users))]
		contentID := weightedContentChoice(r, content, totalWeight)

		watchedAt := time.Now().Add(-time.Duration(r.Intn(1000)) * time.Hour)

		_, _ = stmt.ExecContext(ctx, userID, contentID, watchedAt)
	}

	_ = tx.Commit()
}

func weightedContentChoice(r *rand.Rand, content []ContentItem, total float64) int {
	threshold := r.Float64() * total
	cumulative := 0.0

	for _, c := range content {
		cumulative += c.Popularity
		if cumulative >= threshold {
			return c.ID
		}
	}
	return content[len(content)-1].ID
}

func weightedChoice(r *rand.Rand, options []string, weights []float64) string {
	rn := r.Float64()
	total := 0.0

	for i, w := range weights {
		total += w
		if rn <= total {
			return options[i]
		}
	}
	return options[len(options)-1]
}
