package apperr

import (
	"encoding/json"
)

type Error struct {
	Status  int         `json:"-"`
	Code    interface{} `json:"error"`
	Message string      `json:"message"`
}

func New(status int, code interface{}, message string) *Error {
	return &Error{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func (e *Error) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}
