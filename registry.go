package transform

// RegisterString registers a string-to-string transformation function for key.
// During transformation, fields with a matching tag value will have their
// string value replaced by the result of calling fn.
//
// RegisterString must be called before the first Transform call on this
// Transformer. Calling it after transformation has started panics, since the
// Transformer is immutable once used. Concurrent registration calls are
// serialized; a call that overlaps the first Transform may panic for the
// same reason.
//
// Panics if key is empty or already registered.
//
// Example:
//
//	t.RegisterString("upper", strings.ToUpper)
func (t *Transformer) RegisterString(key string, fn func(string) string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.validateRegistrable()
	if key == "" {
		panic("transform: RegisterString called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterString called with nil function")
	}
	t.validateNewKey(key)
	if t.registry == nil {
		t.registry = make(map[string]stringTransform)
	}
	t.registry[key] = stringTransform{fn: fn}
}

// RegisterStringErr registers a string-to-(string, error) transformation
// function for key. If fn returns an error, the transformation is aborted
// and the error is propagated.
//
// Must be called before the first Transform call. Panics if called
// after transformation has started, if key is empty, or if key is already
// registered. Concurrent registration calls are serialized.
func (t *Transformer) RegisterStringErr(key string, fn func(string) (string, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.validateRegistrable()
	if key == "" {
		panic("transform: RegisterStringErr called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterStringErr called with nil function")
	}
	t.validateNewKey(key)
	if t.registry == nil {
		t.registry = make(map[string]stringTransform)
	}
	t.registry[key] = stringTransform{fnErr: fn}
}

// RegisterBytes registers a []byte-to-[]byte transformation function for key.
// It applies to byte-slice fields — []byte and any named type whose underlying
// type is a slice of bytes — and to containers of those ([][]byte,
// map[K][]byte, [N][]byte, and any nesting of them).
//
// A byte slice is the natural shape for data that is not text: a hash, a
// ciphertext, a protocol frame. RegisterAny can already reach such a field,
// but only by boxing it into an interface on every call and type-asserting it
// back; RegisterBytes calls fn with the slice directly.
//
// fn may return the slice it was given, mutated in place, or a new one. The
// field is set to whatever it returns, including a nil or empty slice.
//
// Byte arrays ([16]byte and the like) are not byte slices and are left alone;
// their length is part of their type, so a function free to return a slice of
// any length cannot write one back. Use RegisterAny for those.
//
// Must be called before the first Transform call. Panics if called
// after transformation has started, if key is empty, or if key is already
// registered. Concurrent registration calls are serialized.
//
// Example:
//
//	t.RegisterBytes("redact", func(b []byte) []byte { return bytes.Repeat([]byte("*"), len(b)) })
func (t *Transformer) RegisterBytes(key string, fn func([]byte) []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.validateRegistrable()
	if key == "" {
		panic("transform: RegisterBytes called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterBytes called with nil function")
	}
	t.validateNewKey(key)
	if t.bytesRegistry == nil {
		t.bytesRegistry = make(map[string]bytesTransform)
	}
	t.bytesRegistry[key] = bytesTransform{fn: fn}
}

// RegisterBytesErr registers a []byte-to-([]byte, error) transformation
// function for key. If fn returns an error, the transformation is aborted and
// the error is propagated.
//
// It applies to the same fields as [Transformer.RegisterBytes].
//
// Must be called before the first Transform call. Panics if called
// after transformation has started, if key is empty, or if key is already
// registered. Concurrent registration calls are serialized.
func (t *Transformer) RegisterBytesErr(key string, fn func([]byte) ([]byte, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.validateRegistrable()
	if key == "" {
		panic("transform: RegisterBytesErr called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterBytesErr called with nil function")
	}
	t.validateNewKey(key)
	if t.bytesRegistry == nil {
		t.bytesRegistry = make(map[string]bytesTransform)
	}
	t.bytesRegistry[key] = bytesTransform{fnErr: fn}
}

// RegisterAny registers a func(any) (any, error) transformation function for
// key. The function receives the field's value as an interface{} and must
// return the transformed value and an optional error.
//
// RegisterAny is the escape hatch for transforming non-string field types
// or implementing complex transformations.
//
// The returned value must be assignable to the field's type; otherwise
// Transform returns an error describing the mismatch. Returning (nil, nil)
// leaves the field unchanged.
//
// Must be called before the first Transform call. Panics if called
// after transformation has started, if key is empty, or if key is already
// registered. Concurrent registration calls are serialized.
func (t *Transformer) RegisterAny(key string, fn func(any) (any, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.validateRegistrable()
	if key == "" {
		panic("transform: RegisterAny called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterAny called with nil function")
	}
	t.validateNewKey(key)
	if t.anyRegistry == nil {
		t.anyRegistry = make(map[string]anyTransform)
	}
	t.anyRegistry[key] = anyTransform{fn: fn}
}

// validateRegistrable panics unless this Transformer can still accept a
// registration. The tag check identifies a zero-value Transformer — New always
// sets a tag and WithTag rejects an empty one — which would otherwise register
// functions that no tag can ever name.
func (t *Transformer) validateRegistrable() {
	if t.tag == "" {
		panic("transform: Transformer must be created with New")
	}
	if t.frozen.Load() {
		panic("transform: cannot register functions after Transform has been called")
	}
}

// validateNewKey panics if key is already registered as any transform type.
// Keys are shared across all three registries: a field's tag is an opaque key
// looked up verbatim, so two registrations under one key would silently shadow
// each other.
func (t *Transformer) validateNewKey(key string) {
	if _, ok := t.registry[key]; ok {
		panic("transform: key already registered: " + key)
	}
	if _, ok := t.bytesRegistry[key]; ok {
		panic("transform: key already registered: " + key)
	}
	if _, ok := t.anyRegistry[key]; ok {
		panic("transform: key already registered: " + key)
	}
}
