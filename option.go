package transform

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
