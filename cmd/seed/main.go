package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/acoshift/pgsql"
	"github.com/acoshift/pgsql/pgctx"
	_ "github.com/lib/pq"
	"github.com/saturnooi/recommendation-service/cmd/seed/config"
)

const (
	userCount         = 2000
	contentCount      = 5000
	watchHistoryCount = 20000
)

type ContentItem struct {
	ID         int64
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

	ctx = pgctx.NewContext(ctx, db)

	if err := truncateTables(ctx); err != nil {
		log.Fatal(err)
	}

	users, err := seedUsers(ctx, r, userCount)
	if err != nil {
		log.Fatal(err)
	}

	content, err := seedContent(ctx, r, contentCount)
	if err != nil {
		log.Fatal(err)
	}

	if err := seedWatchHistory(ctx, r, users, content, watchHistoryCount); err != nil {
		log.Fatal(err)
	}

	log.Println("Seeding completed successfully")
}

func truncateTables(ctx context.Context) error {
	queries := []string{
		"TRUNCATE user_watch_history RESTART IDENTITY CASCADE",
		"TRUNCATE content RESTART IDENTITY CASCADE",
		"TRUNCATE users RESTART IDENTITY CASCADE",
	}

	for _, q := range queries {
		if _, err := pgctx.Exec(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func seedUsers(ctx context.Context, r *rand.Rand, n int) ([]int64, error) {
	const batchSize = 10000

	countries := []string{"US", "GB", "CA", "AU", "DE", "TH", "JP", "FR", "SG", "KR"}
	subscriptions := []string{"free", "basic", "premium"}
	subWeights := []float64{0.5, 0.3, 0.2}
	ids := make([]int64, 0, n)

	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}

		valueStrings := make([]string, 0, end-start)
		valueArgs := make([]any, 0, (end-start)*3)

		argPos := 1

		for i := start; i < end; i++ {

			age := r.Intn(47) + 18
			country := countries[r.Intn(len(countries))]
			sub := weightedChoice(r, subscriptions, subWeights)

			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d)", argPos, argPos+1, argPos+2))

			valueArgs = append(valueArgs, age, country, sub)

			argPos += 3
		}

		query := fmt.Sprintf(`
			INSERT INTO users(age, country, subscription_type)
			VALUES %s
			RETURNING id
		`, strings.Join(valueStrings, ","))

		err := pgctx.Iter(
			ctx,
			func(scan pgsql.Scanner) error {
				var id int64
				if err := scan(&id); err != nil {
					return err
				}
				ids = append(ids, id)
				return nil
			},
			query,
			valueArgs...,
		)
		if err != nil {
			return nil, err
		}
	}

	return ids, nil
}

func seedContent(ctx context.Context, r *rand.Rand, n int) ([]ContentItem, error) {
	const batchSize = 10000

	genres := []string{
		"action", "drama", "comedy", "thriller", "documentary",
		"sci-fi", "romance", "horror", "fantasy", "animation",
	}

	items := make([]ContentItem, 0, n)

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}

		valueStrings := make([]string, 0, end-start)
		valueArgs := make([]any, 0, (end-start)*4)

		argPos := 1

		for i := start; i < end; i++ {

			title := fmt.Sprintf("Movie %d", i+1)
			genre := genres[r.Intn(len(genres))]

			popularity := math.Pow(r.Float64(), 2)

			createdAt := baseTime.AddDate(0, 0, -r.Intn(365))

			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d,$%d)", argPos, argPos+1, argPos+2, argPos+3))

			valueArgs = append(valueArgs, title, genre, popularity, createdAt)

			argPos += 4
		}

		query := fmt.Sprintf(`
			INSERT INTO content(title, genre, popularity_score, created_at)
			VALUES %s
			RETURNING id, popularity_score
		`, strings.Join(valueStrings, ","))

		err := pgctx.Iter(
			ctx,
			func(scan pgsql.Scanner) error {
				var id int64
				var popularity float64

				if err := scan(&id, &popularity); err != nil {
					return err
				}

				items = append(items, ContentItem{
					ID:         id,
					Popularity: popularity,
				})
				return nil
			},
			query,
			valueArgs...,
		)
		if err != nil {
			return nil, err
		}
	}

	return items, nil
}

func seedWatchHistory(ctx context.Context, r *rand.Rand, users []int64, content []ContentItem, n int) error {
	const batchSize = 10000

	baseTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	totalWeight := 0.0
	for _, c := range content {
		totalWeight += c.Popularity
	}

	for start := 0; start < n; start += batchSize {
		end := start + batchSize
		if end > n {
			end = n
		}

		valueStrings := make([]string, 0, end-start)
		valueArgs := make([]any, 0, (end-start)*3)

		argPos := 1

		for i := start; i < end; i++ {

			userID := users[r.Intn(len(users))]
			contentID := weightedContentChoice(r, content, totalWeight)

			watchedAt := baseTime.Add(-time.Duration(r.Intn(1000)) * time.Hour)

			valueStrings = append(valueStrings,
				fmt.Sprintf("($%d,$%d,$%d)", argPos, argPos+1, argPos+2))

			valueArgs = append(valueArgs, userID, contentID, watchedAt)

			argPos += 3
		}

		query := fmt.Sprintf(`
			INSERT INTO user_watch_history(user_id, content_id, watched_at)
			VALUES %s
		`, strings.Join(valueStrings, ","))

		if _, err := pgctx.Exec(ctx, query, valueArgs...); err != nil {
			return err
		}
	}

	return nil
}

func weightedContentChoice(r *rand.Rand, content []ContentItem, total float64) int64 {
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
