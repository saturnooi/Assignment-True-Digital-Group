package httperr

import (
	"net/http"

	"github.com/saturnooi/recommendation-service/internal/errors/apperr"
	"github.com/saturnooi/recommendation-service/internal/errors/codes"
)

func BadRequest(code codes.Code, msg string) *apperr.Error {
	return apperr.New(http.StatusBadRequest, code, msg)
}

func NotFound(code codes.Code, message string) *apperr.Error {
	return apperr.New(http.StatusNotFound, code, message)
}

func Internal(code codes.Code, message string) *apperr.Error {
	return apperr.New(http.StatusInternalServerError, code, message)
}

func ServiceUnavailable(code codes.Code, message string) *apperr.Error {
	return apperr.New(http.StatusServiceUnavailable, code, message)
}

func GatewayTimeout(code codes.Code, message string) *apperr.Error {
	return apperr.New(http.StatusGatewayTimeout, code, message)
}
