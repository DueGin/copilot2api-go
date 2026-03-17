package upstreamerr

import "net/http"

// UpstreamError describes how a specific upstream HTTP status code
// should be rewritten before being returned to the downstream caller.
type UpstreamError struct {
	// StatusCode is the HTTP status code sent to the downstream client.
	StatusCode int
	// Type is a machine-readable error category (e.g. "rate_limit_error").
	Type string
	// Message is the human-readable message shown to the caller.
	Message string
}

// errorMap maps known upstream status codes to downstream error descriptors.
var errorMap = map[int]UpstreamError{
	http.StatusBadRequest: {
		StatusCode: http.StatusBadRequest,
		Type:       "invalid_request_error",
		Message:    "Invalid request parameters",
	},
	http.StatusUnauthorized: {
		StatusCode: http.StatusUnauthorized,
		Type:       "authentication_error",
		Message:    "Authentication failed, please check credentials",
	},
	http.StatusForbidden: {
		StatusCode: http.StatusForbidden,
		Type:       "permission_error",
		Message:    "Permission denied",
	},
	http.StatusNotFound: {
		StatusCode: http.StatusNotFound,
		Type:       "not_found_error",
		Message:    "The requested model or resource was not found",
	},
	http.StatusTooManyRequests: {
		StatusCode: http.StatusTooManyRequests,
		Type:       "rate_limit_error",
		Message:    "Rate limit exceeded, please try again later",
	},
	http.StatusInternalServerError: {
		StatusCode: http.StatusBadGateway,
		Type:       "upstream_error",
		Message:    "Upstream service error, please try again later",
	},
	http.StatusBadGateway: {
		StatusCode: http.StatusBadGateway,
		Type:       "upstream_error",
		Message:    "Upstream service error, please try again later",
	},
	http.StatusServiceUnavailable: {
		StatusCode: http.StatusServiceUnavailable,
		Type:       "upstream_error",
		Message:    "Service temporarily unavailable, please try again later",
	},
}

// defaultError is the fallback for any upstream status code not in errorMap.
var defaultError = UpstreamError{
	StatusCode: http.StatusBadGateway,
	Type:       "upstream_error",
	Message:    "Service unavailable, please contact administrator",
}

// Lookup returns the downstream error descriptor for the given upstream status code.
// Unknown codes fall back to defaultError.
func Lookup(upstreamStatus int) UpstreamError {
	if e, ok := errorMap[upstreamStatus]; ok {
		return e
	}
	return defaultError
}
