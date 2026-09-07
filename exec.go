package transform

import (
	"errors"
	"fmt"
	"reflect"
)

// ========== Closure generation ==========

// compileTypeInfo builds execute closures for every field in the typeInfo,
// then drops fields with no work (unexported/no-op fields and nested types
// with nothing transformable), so the runtime loop iterates exactly the
// fields it will transform. Called once when building a new typeInfo entry.
func (t *Transformer) compileTypeInfo(info *typeInfo) {
	if info.compiled {
		return
	}
	info.compiled = true

	compiled := make([]fieldPlan, 0, len(info.fields))
	for i := range info.fields {
		fp := &info.fields[i]
		if fp.execute = t.compileField(fp); fp.execute != nil {
			compiled = append(compiled, *fp)
		}
	}
	info.fields = compiled
}

// derefField resolves parent.Field(index) through any pointer indirection.
// ok is false when the chain hits a nil pointer, in which case there is
// nothing to transform and the caller returns.
//
// Callers that write to the result must still check CanSet: a field reached
// through an unexported embedded field is readable but not settable.
func derefField(parent reflect.Value, index int) (v reflect.Value, ok bool) {
	f := parent.Field(index)
	for f.Kind() == reflect.Pointer {
		if f.IsNil() {
			return reflect.Value{}, false
		}
		f = f.Elem()
	}
	return f, true
}

func (t *Transformer) compileField(fp *fieldPlan) func(reflect.Value, int) error {
	index := fp.index

	switch fp.kind {
	case kindStrictErr:
		err := fp.err
		return func(reflect.Value, int) error { return err }

	case kindStringDirect:
		fn := fp.strFn
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value, _ int) error {
			f, ok := derefField(parent, index)
			if ok && f.CanSet() {
				f.SetString(fn(f.String()))
			}
			return nil
		}

	case kindStringErrDirect:
		fn := fp.strErr
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value, _ int) error {
			f, ok := derefField(parent, index)
			if !ok || !f.CanSet() {
				return nil
			}
			result, err := fn(f.String())
			if err != nil {
				return err
			}
			f.SetString(result)
			return nil
		}

	case kindStringContainer:
		fn, fnErr := fp.strFn, fp.strErr
		if fn == nil && fnErr == nil {
			return nil
		}
		return func(parent reflect.Value, depth int) error {
			f, ok := derefField(parent, index)
			if !ok || !f.CanSet() {
				return nil
			}
			return applyStringDeep(f, fn, fnErr)
		}

	case kindAnyDirect:
		fn := fp.anyFn
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value, _ int) error {
			f, ok := derefField(parent, index)
			if !ok {
				return nil
			}
			return applyFieldAny(f, fn)
		}

	case kindAnyInterface:
		fn := fp.anyFn
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value, _ int) error {
			f, ok := derefField(parent, index)
			// An embedded unexported interface is read-only and offers no
			// promoted fields to traverse, so it is left alone.
			if !ok || !f.CanSet() || f.IsNil() {
				return nil
			}
			return applyFieldAny(f, fn)
		}

	case kindStruct, kindPtrStruct:
		nested, anyFn := fp.nested, fp.anyFn
		if anyFn == nil && (nested == nil || !nested.has) {
			return nil
		}
		if nested != nil {
			t.compileTypeInfo(nested)
		}
		return func(parent reflect.Value, depth int) error {
			f, ok := derefField(parent, index)
			if !ok {
				return nil
			}
			if anyFn != nil {
				if err := applyFieldAny(f, anyFn); err != nil {
					return err
				}
			}
			if nested != nil && nested.has {
				return t.transformStructDirect(f, nested, depth+1)
			}
			return nil
		}

	case kindSlice, kindArray:
		nested, anyFn := fp.nested, fp.anyFn
		if anyFn == nil && (nested == nil || !nested.has) {
			return nil
		}
		if nested != nil {
			t.compileTypeInfo(nested)
		}
		return func(parent reflect.Value, depth int) error {
			f, ok := derefField(parent, index)
			if !ok {
				return nil
			}
			if anyFn != nil {
				if err := applyFieldAny(f, anyFn); err != nil {
					return err
				}
			}
			return t.transformSlice(f, nested, depth+1)
		}

	case kindMap:
		nested, anyFn := fp.nested, fp.anyFn
		if anyFn == nil && (nested == nil || !nested.has) {
			return nil
		}
		if nested != nil {
			t.compileTypeInfo(nested)
		}
		return func(parent reflect.Value, depth int) error {
			f, ok := derefField(parent, index)
			if !ok {
				return nil
			}
			if anyFn != nil {
				if err := applyFieldAny(f, anyFn); err != nil {
					return err
				}
			}
			return t.transformMap(f, nested, depth+1)
		}

	case kindInterface:
		strFn, strErr := fp.strFn, fp.strErr
		return func(parent reflect.Value, depth int) error {
			f, ok := derefField(parent, index)
			if !ok || !f.CanSet() || f.IsNil() {
				return nil
			}
			// A string transform tagged on an interface field applies when
			// the concrete value turns out to be a string; otherwise the
			// value is traversed like any other.
			if strFn != nil || strErr != nil {
				if inner := f.Elem(); inner.IsValid() && inner.Kind() == reflect.String {
					if strFn != nil {
						return assignAny(strFn(inner.String()), f)
					}
					result, err := strErr(inner.String())
					if err != nil {
						return err
					}
					return assignAny(result, f)
				}
			}
			replacement, err := t.transformElem(f, nil, depth+1)
			if err != nil {
				return err
			}
			if replacement.IsValid() {
				f.Set(replacement)
			}
			return nil
		}

	default: // kindSkip and anything unhandled: no work.
		return nil
	}
}

// applyStringDeep applies a string transform to every string reached from v
// by walking pointers and container elements. It backs kindStringContainer,
// whose element types are known at plan time to bottom out in strings, so the
// recursion is bounded by the type's shape and cannot run away.
func applyStringDeep(v reflect.Value, fn func(string) string, fnErr func(string) (string, error)) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		if !v.CanSet() {
			return nil
		}
		if fn != nil {
			v.SetString(fn(v.String()))
			return nil
		}
		s, err := fnErr(v.String())
		if err != nil {
			return err
		}
		v.SetString(s)
		return nil

	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if err := applyStringDeep(v.Index(i), fn, fnErr); err != nil {
				return err
			}
		}
		return nil

	case reflect.Map:
		// Map values are never addressable, so each one is copied into a
		// reusable buffer, transformed, and written back.
		tmp := reflect.New(v.Type().Elem()).Elem()
		key := reflect.New(v.Type().Key()).Elem()
		iter := v.MapRange()
		for iter.Next() {
			tmp.SetIterValue(iter)
			if err := applyStringDeep(tmp, fn, fnErr); err != nil {
				return err
			}
			key.SetIterKey(iter)
			v.SetMapIndex(key, tmp)
		}
		return nil
	}
	return nil
}

// assignAny assigns the result of a func(any) (any, error) transform to a
// settable value. A nil result is a no-op (the field is left unchanged), so
// a transform cannot be used to nil out a field. A non-nil result that is not
// assignable to the target type returns an error instead of being silently
// dropped.
func assignAny(result any, target reflect.Value) error {
	if result == nil {
		return nil
	}
	rv := reflect.ValueOf(result)
	if !rv.IsValid() {
		return nil
	}
	if !rv.Type().AssignableTo(target.Type()) {
		return fmt.Errorf("%s is not assignable to %s", rv.Type(), target.Type())
	}
	target.Set(rv)
	return nil
}

// applyFieldAny applies a funcAny transform to a settable field value and
// stores the result back. Unsettable fields (e.g. those reached through an
// embedded unexported field) are skipped — their exported sub-fields are
// still traversed by the caller.
func applyFieldAny(f reflect.Value, fn func(any) (any, error)) error {
	if !f.CanSet() {
		return nil
	}
	result, err := fn(f.Interface())
	if err != nil {
		return err
	}
	return assignAny(result, f)
}

// ========== Runtime transform loops ==========

// transformValue transforms val in place, dereferencing pointers and
// interfaces as needed. Internal callers only reach it with an addressable
// value or a reference type (slice, map, pointer); unaddressable structs and
// arrays go through transformElem. TransformValue is the exception: it hands
// over whatever the caller had, so every write below is guarded.
func (t *Transformer) transformValue(val reflect.Value, depth int) error {
	if depth > t.maxDepth {
		return ErrMaxDepth
	}
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return nil
		}
		val = val.Elem()
	}
	if !val.IsValid() {
		return nil
	}
	switch val.Kind() {
	case reflect.Struct:
		return t.transformStructDirect(val, t.getTypeInfo(val.Type()), depth)
	case reflect.Map:
		return t.transformMap(val, nil, depth)
	case reflect.Slice, reflect.Array:
		return t.transformSlice(val, nil, depth)
	}
	return nil
}

// transformElem transforms a value reached through a container: a slice or
// array element, a map value, or an interface field. Values obtained this way
// are frequently unaddressable (map values and interface contents are never
// addressable), so structs and arrays are copied, transformed, and returned
// for the caller to store back. In all other cases the value is transformed
// in place and the returned reflect.Value is invalid.
//
// If info is non-nil it is used for the struct fast path (avoiding a cache
// lookup); callers pass the element type's precomputed plan when available.
func (t *Transformer) transformElem(v reflect.Value, info *typeInfo, depth int) (reflect.Value, error) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return reflect.Value{}, nil
		}
		v = v.Elem()
	}

	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return reflect.Value{}, nil
		}
		inner := v.Elem()
		// A struct or array behind an interface is not addressable: copy it,
		// transform the copy, and hand it back to be stored. Pointers, slices,
		// and maps mutate in place even when reached through an interface.
		if inner.Kind() == reflect.Struct || inner.Kind() == reflect.Array {
			cp := reflect.New(inner.Type()).Elem()
			cp.Set(inner)
			if err := t.transformValue(cp, depth+1); err != nil {
				return reflect.Value{}, err
			}
			return cp, nil
		}
		return reflect.Value{}, t.transformValue(inner, depth+1)
	}

	if v.Kind() == reflect.Struct || v.Kind() == reflect.Array {
		if !v.CanSet() {
			cp := reflect.New(v.Type()).Elem()
			cp.Set(v)
			if err := t.transformValue(cp, depth+1); err != nil {
				return reflect.Value{}, err
			}
			return cp, nil
		}
		if v.Kind() == reflect.Struct && info != nil {
			return reflect.Value{}, t.transformStructDirect(v, info, depth)
		}
	}

	return reflect.Value{}, t.transformValue(v, depth)
}

func (t *Transformer) transformStructDirect(val reflect.Value, info *typeInfo, depth int) error {
	if !info.has {
		return nil
	}
	if depth > t.maxDepth {
		return ErrMaxDepth
	}
	for i := range info.fields {
		err := info.fields[i].execute(val, depth)
		if err == nil {
			continue
		}
		// A depth-limit abort travels back through every frame of a cyclic
		// graph. Wrapping it at each one would build an error string
		// thousands of fields long, so it propagates bare.
		if errors.Is(err, ErrMaxDepth) {
			return err
		}
		rt := val.Type()
		name := rt.Name()
		if name == "" {
			name = rt.String() // anonymous struct type
		}
		return &FieldError{
			Type:  name,
			Field: rt.Field(info.fields[i].index).Name,
			Key:   info.fields[i].key,
			Err:   err,
		}
	}
	return nil
}

// transformSlice walks a slice or array. elemInfo is the plan for the
// element type when the caller already has it (compiled field closures do),
// nil when it has to be looked up.
func (t *Transformer) transformSlice(val reflect.Value, elemInfo *typeInfo, depth int) error {
	if elemInfo == nil {
		elemInfo = t.getTypeInfo(val.Type().Elem())
	}
	if !elemInfo.has {
		return nil
	}
	for j := 0; j < val.Len(); j++ {
		elem := val.Index(j)
		replacement, err := t.transformElem(elem, elemInfo, depth)
		if err != nil {
			return err
		}
		// Slice elements are addressable through the backing array, but the
		// elements of an unaddressable array are not.
		if replacement.IsValid() && elem.CanSet() {
			elem.Set(replacement)
		}
	}
	return nil
}

// transformMap walks a map's values; keys are never transformed. elemInfo is
// the plan for the value type when the caller already has it, nil otherwise.
func (t *Transformer) transformMap(val reflect.Value, elemInfo *typeInfo, depth int) error {
	elemType := val.Type().Elem()
	if elemInfo == nil {
		elemInfo = t.getTypeInfo(elemType)
	}
	if !elemInfo.has {
		return nil
	}

	// Map values are unaddressable, so every value is read into a reusable
	// buffer and written back. Reusing one buffer for the value and one for
	// the key keeps the walk allocation-free per entry.
	tmp := reflect.New(elemType).Elem()
	key := reflect.New(val.Type().Key()).Elem()
	isCopyKind := elemType.Kind() == reflect.Struct || elemType.Kind() == reflect.Array

	iter := val.MapRange()
	for iter.Next() {
		tmp.SetIterValue(iter)

		if isCopyKind {
			var err error
			if elemType.Kind() == reflect.Struct {
				err = t.transformStructDirect(tmp, elemInfo, depth)
			} else {
				err = t.transformValue(tmp, depth)
			}
			if err != nil {
				return err
			}
			key.SetIterKey(iter)
			val.SetMapIndex(key, tmp)
			continue
		}

		// Interface, pointer, slice, or map values. Pointers, slices, and
		// maps mutate through the copied header; interfaces holding a struct
		// or array come back as a replacement to store.
		replacement, err := t.transformElem(tmp, nil, depth)
		if err != nil {
			return err
		}
		if replacement.IsValid() {
			key.SetIterKey(iter)
			val.SetMapIndex(key, replacement)
		}
	}
	return nil
}
