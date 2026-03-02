package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/saturnooi/recommendation-service/internal/errors/codes"
	"github.com/saturnooi/recommendation-service/internal/errors/httperr"
	"github.com/saturnooi/recommendation-service/internal/usecase"
)

type recommendationHandler struct {
	recommendationUsecase usecase.RecommendationUsecase
}

func InitRecommendationHandler(e *echo.Echo, recommendationUsecase usecase.RecommendationUsecase) {
	h := &recommendationHandler{
		recommendationUsecase: recommendationUsecase,
	}

	users := e.Group("/recommendations")
	users.GET("/batch", h.GenerateBatch)
}

func (h *recommendationHandler) GenerateBatch(c echo.Context) error {

	var limit int
	var page int
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

	if pStr := c.QueryParam("page"); pStr != "" {
		p, err := strconv.Atoi(pStr)
		if err != nil {
			return httperr.BadRequest(
				codes.InvalidParameter,
				"Invalid page parameter",
			)
		}
		page = p
	}

	resp, err := h.recommendationUsecase.GenerateBatch(c.Request().Context(), page, limit)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}
