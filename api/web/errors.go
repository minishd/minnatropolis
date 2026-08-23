package web

import (
	"fmt"
)

// Strongly-typed representation of an API error.
//
// Its contents are used to format an HTTP response
// to be read by the frontend
type Error struct {
	Status int
	Note   string
}

func (we *Error) Error() string {
	return fmt.Sprintf("web error %d (%s)", we.Status, we.Note)
}

// Pre-defined errors that may be raised by
// an API endpoint handler.
//
// (Please define any new errors here as well..)
var (
	ErrInvalidCredentials = &Error{400, "invalid credentials"}
	ErrUsernameTaken      = &Error{400, "username taken"}
	ErrUsernameInvalid    = &Error{400, "username invalid"}
	ErrUnauthorized       = &Error{401, "unauthorized"}
	ErrTooManyRequests    = &Error{429, "too many requests"}

	ErrNotJSON        = &Error{415, "only json accepted"}
	ErrBodyMalformed  = &Error{400, "request body malformed"}
	ErrBodyInvalid    = &Error{400, "request body fails validation"}
	ErrServerInternal = &Error{500, "internal server error"}
)
