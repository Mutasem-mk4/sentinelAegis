package models

import "net/http"

// SentinelError represents a structured API error.
type SentinelError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *SentinelError) Error() string {
	return e.Message
}

func NewErrInvalidInput(details string) *SentinelError {
	return &SentinelError{
		Code:    http.StatusBadRequest,
		Message: "Invalid input provided",
		Details: details,
	}
}

func NewErrNotFound(details string) *SentinelError {
	return &SentinelError{
		Code:    http.StatusNotFound,
		Message: "Resource not found",
		Details: details,
	}
}

func NewErrGeminiUnavailable(details string) *SentinelError {
	return &SentinelError{
		Code:    http.StatusServiceUnavailable,
		Message: "Gemini AI analysis unavailable",
		Details: details,
	}
}

func NewErrRateLimited(details string) *SentinelError {
	return &SentinelError{
		Code:    http.StatusTooManyRequests,
		Message: "Rate limit exceeded",
		Details: details,
	}
}

func NewErrInternal(details string) *SentinelError {
	return &SentinelError{
		Code:    http.StatusInternalServerError,
		Message: "Internal server error",
		Details: details,
	}
}
