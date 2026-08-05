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
	ErrServerInternal     = &WebError{500, "internal server error"}
)
