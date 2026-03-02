package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/acoshift/pgsql/pgctx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/saturnooi/recommendation-service/cmd/api/config"
	httpadapter "github.com/saturnooi/recommendation-service/internal/adapter/http"
	"github.com/saturnooi/recommendation-service/internal/adapter/pg"
)

func main() {
	c := config.Init()

	ctx := context.Background()
	db, ctx := pg.NewWithContext(ctx, c.DatabaseURL)
	defer db.Close()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echo.WrapMiddleware(pgctx.Middleware(db)))

	e.Use(middleware.Recover())
	e.HTTPErrorHandler = httpadapter.ErrorHandler

	go func() {
		if err := e.Start(fmt.Sprintf(":%s", c.Port)); err != nil && err != http.ErrServerClosed {
			log.Fatalf("shutting down server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := e.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
}
