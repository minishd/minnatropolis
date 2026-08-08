package weberrors

import "fmt"

// Strongly-typed representation of an API error
type WebError struct {
	Status int
	Note   string
}

func (we *WebError) Error() string {
	return fmt.Sprintf("web error %d (%s)", we.Status, we.Note)
}

var (
	ErrInvalidCredentials = &WebError{400, "invalid credentials"}
	ErrUsernameTaken      = &WebError{400, "username taken"}
	ErrUsernameInvalid    = &WebError{400, "username invalid"}
	ErrServerInternal     = &WebError{500, "internal server error"}
)
