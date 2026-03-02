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
