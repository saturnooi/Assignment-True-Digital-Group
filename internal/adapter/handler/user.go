package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/saturnooi/recommendation-service/internal/errors/codes"
	"github.com/saturnooi/recommendation-service/internal/errors/httperr"
	"github.com/saturnooi/recommendation-service/internal/usecase"
)

type userHandler struct {
	userUsecase usecase.UserUsecase
}

func InitUserHandler(e *echo.Echo, userUsecase usecase.UserUsecase) {
	h := &userHandler{
		userUsecase: userUsecase,
	}

	users := e.Group("/users")
	users.GET("/:id/recommendations", h.GetRecommendations)
	users.POST("/:id/watch-history", h.RecordWatchHistory)
}

func (h *userHandler) GetRecommendations(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return httperr.BadRequest(
			codes.InvalidParameter,
			"Invalid user id",
		)
	}

	var limit int
	if lStr := c.QueryParam("limit"); lStr != "" {
		l, err := strconv.Atoi(lStr)
		if err != nil {
			return httperr.BadRequest(
				codes.InvalidParameter,
				"Invalid limit parameter",
			)
		}
		limit = l
	}

	resp, err := h.userUsecase.GenerateRecommendations(c.Request().Context(), id, limit)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *userHandler) RecordWatchHistory(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return httperr.BadRequest(codes.InvalidParameter, "Invalid user id")
	}

	var req struct {
		ContentID int64 `json:"content_id"`
	}
	if err := c.Bind(&req); err != nil || req.ContentID <= 0 {
		return httperr.BadRequest(codes.InvalidParameter, "Invalid content_id")
	}

	if err := h.userUsecase.RecordWatchHistory(c.Request().Context(), id, req.ContentID); err != nil {
		return err
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
