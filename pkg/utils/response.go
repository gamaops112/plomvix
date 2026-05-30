package utils

import (
	"encoding/json"
	"net/http"
)

const (
	CodeValidationFailed  = "VALIDATION_FAILED"
	CodeUnauthorized      = "UNAUTHORIZED"
	CodeForbidden         = "FORBIDDEN"
	CodeNotFound          = "NOT_FOUND"
	CodeInternalError     = "INTERNAL_ERROR"
	CodeHealthCheckFailed = "HEALTH_CHECK_FAILED"
	CodeServiceUnavail    = "SERVICE_UNAVAILABLE"
)

type Response struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

type ErrorResponse struct {
	Status    string    `json:"status"`
	Error     ErrorBody `json:"error"`
	RequestID string    `json:"request_id,omitempty"`
}

type ErrorBody struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func OK(w http.ResponseWriter, r *http.Request, data interface{}) {
	writeJSON(w, http.StatusOK, Response{
		Status:    "ok",
		Data:      data,
		RequestID: r.Header.Get("X-Request-ID"),
	})
}

func Created(w http.ResponseWriter, r *http.Request, data interface{}) {
	writeJSON(w, http.StatusCreated, Response{
		Status:    "ok",
		Data:      data,
		RequestID: r.Header.Get("X-Request-ID"),
	})
}

func BadRequest(w http.ResponseWriter, r *http.Request, code, message string, details ...string) {
	writeJSON(w, http.StatusBadRequest, ErrorResponse{
		Status:    "error",
		RequestID: r.Header.Get("X-Request-ID"),
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

func Unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	writeJSON(w, http.StatusUnauthorized, ErrorResponse{
		Status:    "error",
		RequestID: r.Header.Get("X-Request-ID"),
		Error: ErrorBody{
			Code:    CodeUnauthorized,
			Message: message,
		},
	})
}

func Forbidden(w http.ResponseWriter, r *http.Request, message string) {
	writeJSON(w, http.StatusForbidden, ErrorResponse{
		Status:    "error",
		RequestID: r.Header.Get("X-Request-ID"),
		Error: ErrorBody{
			Code:    CodeForbidden,
			Message: message,
		},
	})
}

func NotFound(w http.ResponseWriter, r *http.Request, message string) {
	writeJSON(w, http.StatusNotFound, ErrorResponse{
		Status:    "error",
		RequestID: r.Header.Get("X-Request-ID"),
		Error: ErrorBody{
			Code:    CodeNotFound,
			Message: message,
		},
	})
}

func InternalError(w http.ResponseWriter, r *http.Request, message string) {
	writeJSON(w, http.StatusInternalServerError, ErrorResponse{
		Status:    "error",
		RequestID: r.Header.Get("X-Request-ID"),
		Error: ErrorBody{
			Code:    CodeInternalError,
			Message: message,
		},
	})
}

func ServiceUnavailable(w http.ResponseWriter, r *http.Request, code, message string, details ...string) {
	writeJSON(w, http.StatusServiceUnavailable, ErrorResponse{
		Status:    "error",
		RequestID: r.Header.Get("X-Request-ID"),
		Error: ErrorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}
