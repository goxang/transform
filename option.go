package transform

// DefaultMaxDepth is the traversal depth limit applied unless WithMaxDepth
// overrides it. It exists to turn cyclic data — a pointer chain that loops
// back on itself — into an error instead of an unrecoverable stack overflow.
// The limit is far above any realistic struct nesting.
const DefaultMaxDepth = 1000

// Option configures a Transformer at construction time.
type Option func(*Transformer)

// WithTag sets the struct tag name used to look up transformation keys.
// The default tag is "transform".
//
// Panics if tag is empty.
//
// Example:
//
//	t := transform.New(transform.WithTag("xform"))
//	// Now uses `xform:"key"` struct tags
func WithTag(tag string) Option {
	if tag == "" {
		panic("transform: WithTag called with empty tag")
	}
	return func(t *Transformer) {
		t.tag = tag
	}
}

// WithStrict makes Transform fail on a tag whose key is not registered, or
// that sits on a field the key cannot be applied to.
//
// By default an unresolvable tag is silently ignored, which turns a typo
// (`transform:"uppr"`) or a json-style tag option (`transform:"upper,omitempty"`)
// into a no-op that looks like a working transformation. Strict mode reports
// these as an ErrUnknownKey or ErrUnusableKey wrapped in a *FieldError.
//
// The check runs when the type's plan is built, so it costs nothing on the
// hot path after the first Transform of a given type.
//
// Example:
//
//	t := transform.New(transform.WithStrict())
func WithStrict() Option {
	return func(t *Transformer) {
		t.strict = true
	}
}

// WithMaxDepth sets the traversal depth limit. Exceeding it aborts the
// transformation with ErrMaxDepth rather than letting recursion run into a
// stack overflow, which Go cannot recover from.
//
// The default is DefaultMaxDepth. Panics if depth is not positive.
func WithMaxDepth(depth int) Option {
	if depth <= 0 {
		panic("transform: WithMaxDepth called with non-positive depth")
	}
	return func(t *Transformer) {
		t.maxDepth = depth
	}
}
