package apperror

import (
	"errors"
	"net/http"
)

type Error struct {
	Code, Message string
	Status        int
	Err           error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}
func (e *Error) Unwrap() error { return e.Err }
func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}
func Wrap(err error, status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message, Err: err}
}

var (
	ErrNotFound     = New(http.StatusNotFound, "not_found", "resource not found")
	ErrUnauthorized = New(http.StatusUnauthorized, "unauthorized", "authentication required")
	ErrForbidden    = New(http.StatusForbidden, "forbidden", "insufficient permissions")
	ErrConflict     = New(http.StatusConflict, "conflict", "resource conflict")
	ErrValidation   = New(http.StatusBadRequest, "validation_error", "request validation failed")
)

func As(err error) *Error {
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	return &Error{Status: http.StatusInternalServerError, Code: "internal_error", Message: "an unexpected error occurred", Err: err}
}
