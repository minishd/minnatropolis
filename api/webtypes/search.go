package webtypes

import "github.com/minishd/minnatropolis/datastore"

// Shared sort direction type for API handlers
type SortDirection string

const (
	SortAsc  SortDirection = "asc"
	SortDesc SortDirection = "desc"
)

func (so *SortDirection) UnmarshalText(b []byte) error {
	// Make sure it's a valid variant
	switch SortDirection(b) {
	case SortAsc, SortDesc:
		*so = SortDirection(b)
		return nil
	default:
		return ErrInvalidEnum
	}
}

func (so SortDirection) ToDS() datastore.SortDirection {
	switch so {
	case SortAsc:
		return datastore.SortAscending
	case SortDesc:
		return datastore.SortDescending
	default:
		panic("invalid sort-direction to datastore")
	}
}
