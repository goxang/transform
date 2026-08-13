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
	t.validateNoRegisterAfterUse()
	if key == "" {
		panic("transform: RegisterString called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterString called with nil function")
	}
	t.validateNewKey(key)
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
	t.validateNoRegisterAfterUse()
	if key == "" {
		panic("transform: RegisterStringErr called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterStringErr called with nil function")
	}
	t.validateNewKey(key)
	t.registry[key] = stringTransform{fnErr: fn}
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
	t.validateNoRegisterAfterUse()
	if key == "" {
		panic("transform: RegisterAny called with empty key")
	}
	if fn == nil {
		panic("transform: RegisterAny called with nil function")
	}
	t.validateNewKey(key)
	t.anyRegistry[key] = anyTransform{fn: fn}
}

func (t *Transformer) validateNoRegisterAfterUse() {
	if t.frozen.Load() {
		panic("transform: cannot register functions after Transform has been called")
	}
}

// validateNewKey panics if key is already registered as any transform type.
// Keys are shared across the string and any registries: a field's tag is an
// opaque key looked up verbatim, so two registrations under one key would
// silently shadow each other.
func (t *Transformer) validateNewKey(key string) {
	if _, ok := t.registry[key]; ok {
		panic("transform: key already registered: " + key)
	}
	if _, ok := t.anyRegistry[key]; ok {
		panic("transform: key already registered: " + key)
	}
}
