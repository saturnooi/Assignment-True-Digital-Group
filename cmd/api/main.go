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
	"github.com/saturnooi/recommendation-service/internal/adapter/cache"
	"github.com/saturnooi/recommendation-service/internal/adapter/handler"
	httpadapter "github.com/saturnooi/recommendation-service/internal/adapter/http"
	"github.com/saturnooi/recommendation-service/internal/adapter/model"
	"github.com/saturnooi/recommendation-service/internal/adapter/pg"
	"github.com/saturnooi/recommendation-service/internal/adapter/repository"
	"github.com/saturnooi/recommendation-service/internal/usecase"
)

func main() {
	c := config.Init()

	ctx := context.Background()
	db, ctx := pg.NewWithContext(ctx, c.DatabaseURL)
	defer db.Close()

	rc := cache.New(ctx, c.RedisURL, c.RedisPassword, c.RedisDB)
	defer rc.Close()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echo.WrapMiddleware(pgctx.Middleware(db)))
	e.Use(middleware.Recover())
	e.HTTPErrorHandler = httpadapter.ErrorHandler

	userRepo := repository.NewUserRepository()
	modelClient := model.NewScoringClient()
	cacheClient := cache.NewRedisCache(rc)
	userUsecase := usecase.NewUserUsecase(userRepo, modelClient, cacheClient)
	recommendationUsecase := usecase.NewRecommendationUsecase(userRepo, modelClient)

	handler.InitUserHandler(e, userUsecase)
	handler.InitRecommendationHandler(e, recommendationUsecase)

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
