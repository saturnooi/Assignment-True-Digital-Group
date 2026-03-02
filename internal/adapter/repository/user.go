package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/acoshift/pgsql"
	"github.com/acoshift/pgsql/pgctx"
	"github.com/lib/pq"
	"github.com/saturnooi/recommendation-service/internal/domain/user"
	"github.com/saturnooi/recommendation-service/internal/port"
)

type userRepo struct{}

func NewUserRepository() port.UserRepository {
	return &userRepo{}
}

func (r *userRepo) GetUserByID(ctx context.Context, id int64) (*user.User, error) {
	var result user.User

	err := pgctx.QueryRow(ctx,
		`SELECT 
			id, 
			age, 
			country, 
			subscription_type 
		FROM 
			users 
		WHERE 
			id=$1`,
		id,
	).Scan(
		&result.ID,
		&result.Age,
		&result.Country,
		&result.Subscription,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &result, err
}

func (r *userRepo) GetUserWatchHistory(ctx context.Context, id int64) ([]user.WatchRecord, error) {
	var results []user.WatchRecord

	err := pgctx.Iter(
		ctx,
		func(scan pgsql.Scanner) error {
			var x user.WatchRecord
			err := scan(
				&x.Genre,
				&x.WatchedAt,
			)
			if err != nil {
				return err
			}
			results = append(results, x)
			return nil
		},
		`SELECT 
			c.genre, 
			uwh.watched_at
		FROM 
			user_watch_history uwh
		JOIN 
			content c ON c.id = uwh.content_id
		WHERE 
			uwh.user_id=$1
		ORDER BY 
			uwh.watched_at DESC
		LIMIT 
			50`,
		id,
	)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (r *userRepo) GetUnwatchedContent(ctx context.Context, id int64) ([]user.Content, error) {
	var results []user.Content

	err := pgctx.Iter(
		ctx,
		func(scan pgsql.Scanner) error {
			var x user.Content
			err := scan(
				&x.ID,
				&x.Title,
				&x.Genre,
				&x.PopularityScore,
				&x.CreatedAt,
			)
			if err != nil {
				return err
			}
			results = append(results, x)
			return nil
		},
		`SELECT 
				id, 
				title, 
				genre, 
				popularity_score, 
				created_at
		FROM
			content 
		WHERE
			NOT EXISTS (
				SELECT 1 
				FROM user_watch_history uwh 
				WHERE uwh.user_id = $1 AND uwh.content_id = content.id
			)
		ORDER BY 
			popularity_score DESC
		LIMIT
			100
		`, id,
	)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (r *userRepo) GetUserIDsPaginated(ctx context.Context, limit, offset int) ([]int64, int, error) {

	var ids []int64
	var total int

	err := pgctx.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	err = pgctx.Iter(ctx, func(scan pgsql.Scanner) error {
		var id int64
		if err := scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
		return nil
	},
		`SELECT id FROM users ORDER BY id LIMIT $1 OFFSET $2`,
		limit, offset,
	)

	if err != nil {
		return nil, 0, err
	}

	return ids, total, nil
}

func (r *userRepo) GetWatchHistoryByUserIDs(ctx context.Context, userIDs []int64) (map[int64][]user.WatchRecord, error) {

	if len(userIDs) == 0 {
		return map[int64][]user.WatchRecord{}, nil
	}

	query := `
        SELECT 
            uwh.user_id, 
            c.genre, 
            uwh.watched_at,
            uwh.content_id
        FROM 
            user_watch_history uwh
        JOIN 
            content c ON c.id = uwh.content_id
        WHERE 
            uwh.user_id = ANY($1)
        ORDER BY 
            uwh.user_id, uwh.watched_at DESC
    `

	results := make(map[int64][]user.WatchRecord)

	err := pgctx.Iter(ctx, func(scan pgsql.Scanner) error {
		var userID int64
		var genre string
		var watchedAt time.Time
		var contentID int64
		if err := scan(
			&userID,
			&genre,
			&watchedAt,
			&contentID,
		); err != nil {
			return err
		}
		results[userID] = append(results[userID], user.WatchRecord{
			ContentID: contentID,
			Genre:     genre,
			WatchedAt: watchedAt,
		})
		return nil
	},
		query, pq.Array(userIDs),
	)

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (r *userRepo) GetTopContent(ctx context.Context) ([]user.Content, error) {

	query := `
        SELECT id, title, genre, popularity_score, created_at
        FROM content
        ORDER BY popularity_score DESC
        LIMIT 100
    `

	var results []user.Content

	err := pgctx.Iter(ctx, func(scan pgsql.Scanner) error {
		var c user.Content
		if err := scan(
			&c.ID,
			&c.Title,
			&c.Genre,
			&c.PopularityScore,
			&c.CreatedAt,
		); err != nil {
			return err
		}
		results = append(results, c)
		return nil
	}, query)

	return results, err
}
