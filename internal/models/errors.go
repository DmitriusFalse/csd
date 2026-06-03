package models

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type ErrorCode string

const (
	ErrCodeUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden          ErrorCode = "FORBIDDEN"
	ErrCodeNotFound           ErrorCode = "NOT_FOUND"
	ErrCodeRateLimited        ErrorCode = "RATE_LIMITED"
	ErrCodeCloudflare         ErrorCode = "CLOUDFLARE"
	ErrCodeNetwork            ErrorCode = "NETWORK"
	ErrCodeDownloadIncomplete ErrorCode = "DOWNLOAD_INCOMPLETE"
	ErrCodeServerError        ErrorCode = "SERVER_ERROR"
	ErrCodeInvalidRequest     ErrorCode = "INVALID_REQUEST"
)

type APIError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	StatusCode int       `json:"statusCode"`
	RetryAfter int       `json:"retryAfter,omitempty"`
	Retryable  bool      `json:"retryable"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func NewAPIError(code ErrorCode, message string, statusCode int, retryable bool) *APIError {
	return &APIError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Retryable:  retryable,
	}
}

func NewRetryableError(code ErrorCode, message string, statusCode int, retryAfter int) *APIError {
	return &APIError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		RetryAfter: retryAfter,
		Retryable:  true,
	}
}

func ClassifyHTTPError(statusCode int, body string) *APIError {
	switch statusCode {
	case http.StatusUnauthorized:
		return NewAPIError(
			ErrCodeUnauthorized,
			"Неверный API-ключ. Проверьте ключ в настройках.",
			statusCode, false,
		)
	case http.StatusForbidden:
		return NewAPIError(
			ErrCodeForbidden,
			"Нет доступа к модели. Возможно, модель приватная или удалена.",
			statusCode, false,
		)
	case http.StatusNotFound:
		return NewAPIError(
			ErrCodeNotFound,
			"Модель или файл не найдены. Проверьте корректность ID.",
			statusCode, false,
		)
	case http.StatusTooManyRequests:
		retryAfter := 60
		return NewRetryableError(
			ErrCodeRateLimited,
			fmt.Sprintf("Civitai ограничил запросы. Повтор через %d секунд.", retryAfter),
			statusCode, retryAfter,
		)
	case http.StatusServiceUnavailable:
		return NewRetryableError(
			ErrCodeCloudflare,
			"Cloudflare защита. Добавлена задержка, повтор...",
			statusCode, 30,
		)
	default:
		if statusCode >= 500 {
			return NewRetryableError(
				ErrCodeServerError,
				fmt.Sprintf("Ошибка сервера Civitai (HTTP %d). Повтор...", statusCode),
				statusCode, 30,
			)
		}
		return NewAPIError(
			ErrCodeServerError,
			fmt.Sprintf("Неожиданный ответ сервера (HTTP %d)", statusCode),
			statusCode, false,
		)
	}
}

func IsRetryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable
	}
	return false
}

func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}
	// Fallback: check error message for wrapped API errors
	msg := err.Error()
	return strings.Contains(msg, "connection") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "reset")
}

func IsDownloadCanceled(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "canceled") || strings.Contains(msg, "context canceled")
}
