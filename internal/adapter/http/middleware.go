package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/saturnooi/recommendation-service/internal/errors/apperr"
)

func ErrorHandler(err error, c echo.Context) {

	if c.Response().Committed {
		return
	}

	var code int
	var body interface{}

	if appErr, ok := err.(*apperr.Error); ok {
		code = appErr.Status
		body = appErr
	} else {
		code = http.StatusInternalServerError
		body = map[string]interface{}{
			"error":   "internal_error",
			"message": "An unexpected error occurred",
		}
	}

	c.Response().WriteHeader(code)
	_ = c.JSON(code, body)
}
