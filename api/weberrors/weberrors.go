package weberrors

import (
	"fmt"
)

// Strongly-typed representation of an API error.
//
// Its contents are used to format an HTTP response
// to be read by the frontend
type WebError struct {
	Status int
	Note   string
}

func (we *WebError) Error() string {
	return fmt.Sprintf("web error %d (%s)", we.Status, we.Note)
}

// Pre-defined errors that may be raised by
// an API endpoint handler.
//
// (Please define any new errors here as well..)
var (
	ErrInvalidCredentials = &WebError{400, "invalid credentials"}
	ErrUsernameTaken      = &WebError{400, "username taken"}
	ErrUsernameInvalid    = &WebError{400, "username invalid"}

	ErrNotJSON        = &WebError{415, "only json accepted"}
	ErrBodyMalformed  = &WebError{400, "request body malformed"}
	ErrBodyInvalid    = &WebError{400, "request body fails validation"}
	ErrServerInternal = &WebError{500, "internal server error"}
)
