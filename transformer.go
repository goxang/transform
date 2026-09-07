package transform

import (
	"reflect"
	"sync"
	"sync/atomic"
)

// Transformer transforms struct field values using registered transformation
// functions.
//
// A Transformer must be created with New; the zero value is not usable:
// Transform is a no-op and the Register methods panic on nil registries.
//
// It is safe for concurrent use after all functions have been registered.
// Registering concurrently with a running Transform is serialized against
// the freeze check, but a Register call that loses the race with the first
// Transform panics.
type Transformer struct {
	tag         string
	strict      bool
	maxDepth    int
	registry    map[string]stringTransform
	anyRegistry map[string]anyTransform

	// mu guards registry and anyRegistry. Registration takes the write
	// lock; plan building snapshots both maps under the read lock, so a
	// plan is always compiled against one consistent set of functions and
	// never races with a concurrent registration.
	mu     sync.RWMutex
	cache  sync.Map // reflect.Type → *typeInfo
	frozen atomic.Bool
}

// New creates a new Transformer with the given options.
func New(opts ...Option) *Transformer {
	t := &Transformer{
		tag:         "transform",
		maxDepth:    DefaultMaxDepth,
		registry:    make(map[string]stringTransform),
		anyRegistry: make(map[string]anyTransform),
	}
	for _, o := range opts {
		if o != nil {
			o(t)
		}
	}
	return t
}

// Transform applies registered transformation functions to obj in place.
// obj must be a non-nil pointer whose underlying value is a struct, slice,
// array, or map; anything else returns ErrInvalidSrc.
//
// Struct fields tagged with the Transformer's tag (default "transform") are
// matched to registered functions by the tag value. Fields without a matching
// tag or registered function are left unchanged — see WithStrict to make an
// unresolvable tag an error instead.
//
// Transform recurses into nested structs, pointers, slices, arrays, and maps.
// Nested struct fields are also checked for transformation tags. Map keys are
// never transformed, only values.
//
// Transform stops at the first error. If a transform function returns an
// error, the error is wrapped in a *FieldError and returned immediately.
// Fields processed before the error remain transformed; fields after are
// untouched. There is no partial rollback.
//
// Recursive types are supported, including mutually recursive ones. Cyclic
// data — a pointer, slice, map, or interface chain that loops back on itself —
// is bounded by the traversal depth limit and reported as ErrMaxDepth rather
// than recursing until the stack gives out. See WithMaxDepth.
//
// The first call freezes the Transformer: any later Register call panics.
// After that a Transformer is safe for concurrent use.
func (t *Transformer) Transform(obj any) error {
	if err := validateTransformSrc(obj); err != nil {
		return err
	}
	t.frozen.Store(true)
	return t.transformValue(reflect.ValueOf(obj), 0)
}

// ========== internal helpers ==========

func validateTransformSrc(obj any) error {
	val := reflect.ValueOf(obj)
	if val.Kind() != reflect.Ptr || val.IsNil() {
		return ErrInvalidSrc
	}
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ErrInvalidSrc
		}
		val = val.Elem()
	}
	switch val.Kind() {
	case reflect.Struct, reflect.Slice, reflect.Array, reflect.Map:
		return nil
	default:
		return ErrInvalidSrc
	}
}

type stringTransform struct {
	fn    func(string) string
	fnErr func(string) (string, error)
}

type anyTransform struct {
	fn func(any) (any, error)
}
