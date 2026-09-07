package transform

import (
	"errors"
	"fmt"
)

// ErrInvalidSrc is returned when Transform receives a src argument that cannot
// be resolved to a settable struct, slice, array, or map.
var ErrInvalidSrc = errors.New("src must be a non-nil pointer to a struct, slice, array, or map")

// ErrMaxDepth is returned when traversal exceeds the configured depth limit.
// In practice this means the value graph is cyclic: a pointer, slice, map, or
// interface chain that loops back on itself. See WithMaxDepth.
var ErrMaxDepth = errors.New("maximum traversal depth exceeded (cyclic data?)")

// ErrUnknownKey is reported by a Transformer built WithStrict when a field's
// tag names a key that was never registered.
var ErrUnknownKey = errors.New("no transformation registered for key")

// ErrUnusableKey is reported by a Transformer built WithStrict when a field's
// tag names a registered key that cannot be applied to that field's type —
// for example a func(string) string on an int field.
var ErrUnusableKey = errors.New("registered transformation does not apply to this field type")

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
