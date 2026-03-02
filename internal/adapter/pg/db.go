package pg

import (
	"context"
	"database/sql"
	"log"
	"log/slog"
	"runtime"
	"time"

	"contrib.go.opencensus.io/integrations/ocsql"
	"github.com/acoshift/pgsql/pgctx"
	"github.com/leporo/sqlf"
)

func New(uri string) *sql.DB {
	driver, _ := ocsql.Register("postgres", ocsql.WithQuery(true))
	db, err := sql.Open(driver, uri)
	if err != nil {
		log.Fatalln("pg: cannot open Postgres driver", err.Error())
	}

	err = db.Ping()
	if err != nil {
		log.Fatalln("pg: cannot connect to Postgres:", err.Error())
	}
	slog.Info("pg: Postgres connected")

	sqlf.SetDialect(sqlf.PostgreSQL)

	maxConn := runtime.NumCPU() * 4
	db.SetMaxOpenConns(maxConn)
	db.SetMaxIdleConns(maxConn)
	return db
}

func NewWithContext(ctx context.Context, uri string) (*sql.DB, context.Context) {
	db := New(uri)
	return db, pgctx.NewContext(ctx, db)
}

type TxOptions struct {
	MaxAttempts int
	Interval    time.Duration
	Isolation   sql.IsolationLevel
}
