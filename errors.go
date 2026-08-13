package transform

import (
	"errors"
	"fmt"
)

// ErrInvalidSrc is returned when Transform receives a src argument that cannot
// be resolved to a settable struct, slice, array, or map.
var ErrInvalidSrc = errors.New("src must be a non-nil pointer to a struct, slice, array, or map")

// A FieldError describes a failure that occurred while transforming one field
// of a struct.
type FieldError struct {
	Type  string // name of the struct type
	Field string // name of the field that failed
	Key   string // transformation key (if any)
	Err   error  // underlying error
}

func (e *FieldError) Error() string {
	if e.Key != "" {
		return fmt.Sprintf("transform %s.%s (key %q): %v", e.Type, e.Field, e.Key, e.Err)
	}
	return fmt.Sprintf("transform %s.%s: %v", e.Type, e.Field, e.Err)
}

func (e *FieldError) Unwrap() error {
	return e.Err
}
