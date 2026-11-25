package webtypes

// This package contains models to be used by request or response types
// Some types may map 1-to-1, but we still want to keep them separate
// from datastore types...
// The ones here will usually implement MarshalJSON and such

import (
	"fmt"
)

type UnmarshalError struct{ reason string }

var (
	ErrInvalidEnum = &UnmarshalError{"enum value is invalid"}
)

func (ue *UnmarshalError) Error() string {
	return fmt.Sprintf("unmarshal error (%s)", ue.reason)
}
