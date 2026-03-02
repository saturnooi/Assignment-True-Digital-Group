package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/acoshift/pgsql"
	"github.com/acoshift/pgsql/pgctx"
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
