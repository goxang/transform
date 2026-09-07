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
	val := reflect.ValueOf(obj)
	if err := validateTransformSrc(val); err != nil {
		return err
	}
	t.frozen.Store(true)
	return t.transformValue(val, 0)
}

// TransformValue applies registered transformation functions in place to the
// value val refers to. It is the reflect.Value form of Transform, for callers
// that already hold a reflect.Value and would otherwise have to box it back
// into an interface to call Transform.
//
// Pointers and interfaces are followed. A struct, slice, array, or map at the
// end of that chain is transformed; any other kind is left alone, as are nil
// pointers, nil interfaces, and the zero reflect.Value. Unlike Transform,
// TransformValue never returns ErrInvalidSrc: a reflect.Value gives a caller
// walking a value graph no way to know in advance which nodes are
// transformable, so an uninteresting value is nothing to do rather than a
// mistake.
//
// Writing a field back requires a settable destination. reflect.ValueOf
// returns an unaddressable value, so the fields of a struct passed that way
// are skipped, exactly as they would be if the struct had been passed to a
// function by value; pass a pointer, or an addressable value such as
// reflect.ValueOf(&v).Elem(), to have them transformed. Slices and maps are
// references, so their elements are transformed either way.
//
// Everything else — traversal, tags, errors, the depth limit — matches
// Transform, including that the first call freezes the Transformer.
func (t *Transformer) TransformValue(val reflect.Value) error {
	t.frozen.Store(true)
	return t.transformValue(val, 0)
}

// ========== internal helpers ==========

// validateTransformSrc reports whether val is something Transform can write
// through: a non-nil pointer chain ending in a struct, slice, array, or map.
// It takes the reflect.Value rather than the interface so that Transform
// unpacks the interface once and hands the same value to the walk.
func validateTransformSrc(val reflect.Value) error {
	if val.Kind() != reflect.Pointer {
		return ErrInvalidSrc
	}
	for val.Kind() == reflect.Pointer {
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
