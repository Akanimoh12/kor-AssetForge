package apperrors

import (
	"errors"
	"fmt"
	"net/http"
)

type ErrorCode string

const (
	CodeInternalServerError  ErrorCode = "internal_server_error"
	CodeBadRequest           ErrorCode = "bad_request"
	CodeValidationFailed     ErrorCode = "validation_failed"
	CodeNotFound             ErrorCode = "not_found"
	CodeDatabaseError        ErrorCode = "database_error"
	CodeExternalServiceError ErrorCode = "external_service_error"
	CodeUnauthorized         ErrorCode = "unauthorized"
)

type AppError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Status  int       `json:"status"`
	Err     error     `json:"-"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(code ErrorCode, message string, status int) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
	}
}

func Wrap(err error, code ErrorCode, message string, status int) *AppError {
	if err == nil {
		return New(code, message, status)
	}
	return &AppError{
		Code:    code,
		Message: message,
		Status:  status,
		Err:     err,
	}
}

func IsAppError(err error) bool {
	var appErr *AppError
	return errors.As(err, &appErr)
}

func FromError(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return Wrap(err, CodeInternalServerError, "An unexpected server error occurred", http.StatusInternalServerError)
}

type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Code      ErrorCode `json:"code"`
	RequestID string    `json:"request_id"`
	Debug     string    `json:"debug,omitempty"`
}

func FormatErrorResponse(err *AppError, requestID string, debug bool) ErrorResponse {
	if err == nil {
		err = New(CodeInternalServerError, "An unexpected server error occurred", http.StatusInternalServerError)
	}

	if err.Status == 0 {
		err.Status = http.StatusInternalServerError
	}

	resp := ErrorResponse{
		Error:     http.StatusText(err.Status),
		Message:   err.Message,
		Code:      err.Code,
		RequestID: requestID,
	}

	if debug && err.Err != nil {
		resp.Debug = err.Err.Error()
	}

	return resp
}
