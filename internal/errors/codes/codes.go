package codes

type Code string

const (
	InvalidParameter Code = "invalid_parameter"
	UserNotFound     Code = "user_not_found"
	ModelUnavailable Code = "model_unavailable"
	InternalError    Code = "internal_error"
	Unknown          Code = "unknown"
	RequestTimeout   Code = "request_timeout"
)
