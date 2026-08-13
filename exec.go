package transform

import (
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

func (t *Transformer) compileField(fp *fieldPlan) func(reflect.Value) error {
	if fp.kind == kindSkip || fp.kind == kindNoOp {
		return nil
	}

	index := fp.index

	switch fp.kind {
	case kindStringDirect:
		fn := fp.strFn
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			if f.CanSet() {
				f.SetString(fn(f.String()))
			}
			return nil
		}

	case kindStringErrDirect:
		fn := fp.strErr
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			if !f.CanSet() {
				return nil
			}
			result, err := fn(f.String())
			if err != nil {
				return err
			}
			f.SetString(result)
			return nil
		}

	case kindAnyDirect:
		fn := fp.anyFn
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			if !f.CanSet() {
				return nil
			}
			result, err := fn(f.Interface())
			if err != nil {
				return err
			}
			return assignAny(result, f)
		}

	case kindAnyInterface:
		fn := fp.anyFn
		if fn == nil {
			return nil
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			// An embedded unexported interface is read-only and offers no
			// promoted fields to traverse, so it is left alone.
			if !f.CanSet() {
				return nil
			}
			if f.IsNil() {
				return nil
			}
			result, err := fn(f.Interface())
			if err != nil {
				return err
			}
			return assignAny(result, f)
		}

	case kindStruct:
		nested := fp.nested
		anyFn := fp.anyFn
		if anyFn == nil && (nested == nil || !nested.has) {
			return nil
		}
		if nested != nil {
			t.compileTypeInfo(nested)
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			if anyFn != nil {
				if err := applyFieldAny(f, anyFn); err != nil {
					return err
				}
			}
			if nested != nil && nested.has {
				return t.transformStructDirect(f, nested)
			}
			return nil
		}

	case kindPtrStruct:
		nested := fp.nested
		anyFn := fp.anyFn
		if anyFn == nil && (nested == nil || !nested.has) {
			return nil
		}
		if nested != nil {
			t.compileTypeInfo(nested)
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			if anyFn != nil {
				if err := applyFieldAny(f, anyFn); err != nil {
					return err
				}
			}
			if nested != nil && nested.has {
				return t.transformStructDirect(f, nested)
			}
			return nil
		}

	case kindSlice, kindArray:
		nested := fp.nested
		anyFn := fp.anyFn
		if anyFn == nil && (nested == nil || !nested.has) {
			return nil
		}
		if nested != nil {
			t.compileTypeInfo(nested)
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			if anyFn != nil {
				if err := applyFieldAny(f, anyFn); err != nil {
					return err
				}
			}
			return t.transformSlice(f)
		}

	case kindMap:
		nested := fp.nested
		anyFn := fp.anyFn
		if anyFn == nil && (nested == nil || !nested.has) {
			return nil
		}
		if nested != nil {
			t.compileTypeInfo(nested)
		}
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			if anyFn != nil {
				if err := applyFieldAny(f, anyFn); err != nil {
					return err
				}
			}
			return t.transformMap(f)
		}

	case kindInterface:
		strFn := fp.strFn
		strErr := fp.strErr
		// Plain interface field (no string transform): the original path with
		// no extra work.
		if strFn == nil && strErr == nil {
			return func(parent reflect.Value) error {
				f := parent.Field(index)
				for f.Kind() == reflect.Ptr {
					if f.IsNil() {
						return nil
					}
					f = f.Elem()
				}
				if !f.CanSet() {
					return nil
				}
				if f.IsNil() {
					return nil
				}
				replacement, err := t.transformElem(f, nil)
				if err != nil {
					return err
				}
				if replacement.IsValid() {
					f.Set(replacement)
				}
				return nil
			}
		}
		// Interface field with a registered string transform: apply it when the
		// concrete value is a string, otherwise traverse.
		return func(parent reflect.Value) error {
			f := parent.Field(index)
			for f.Kind() == reflect.Ptr {
				if f.IsNil() {
					return nil
				}
				f = f.Elem()
			}
			if !f.CanSet() {
				return nil
			}
			if f.IsNil() {
				return nil
			}
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
			replacement, err := t.transformElem(f, nil)
			if err != nil {
				return err
			}
			if replacement.IsValid() {
				f.Set(replacement)
			}
			return nil
		}

	default:
		return nil
	}
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
// stores the result back. It is used for compound fields (struct/slice/map)
// that carry a transform in addition to being traversed. Unsettable fields
// (e.g. embedded unexported fields) are skipped — their exported sub-fields
// are still traversed.
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
// interfaces as needed. val must be addressable or a reference type (slice,
// map, pointer); it is never called directly with an unaddressable struct or
// array — those go through transformElem.
func (t *Transformer) transformValue(val reflect.Value) error {
	for val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
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
		return t.transformStruct(val)
	case reflect.Map:
		return t.transformMap(val)
	case reflect.Slice, reflect.Array:
		return t.transformSlice(val)
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
func (t *Transformer) transformElem(v reflect.Value, info *typeInfo) (reflect.Value, error) {
	for v.Kind() == reflect.Ptr {
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
		// A pointer behind an interface is still addressable.
		if inner.Kind() == reflect.Ptr {
			return reflect.Value{}, t.transformValue(inner)
		}
		// A struct or array behind an interface is not addressable: copy it.
		if inner.Kind() == reflect.Struct || inner.Kind() == reflect.Array {
			cp := reflect.New(inner.Type()).Elem()
			cp.Set(inner)
			if err := t.transformValue(cp); err != nil {
				return reflect.Value{}, err
			}
			return cp, nil
		}
		// Slices and maps mutate in place even when reached via an interface.
		return reflect.Value{}, t.transformValue(inner)
	}

	if v.Kind() == reflect.Struct {
		if v.CanSet() {
			if info != nil {
				return reflect.Value{}, t.transformStructDirect(v, info)
			}
			return reflect.Value{}, t.transformStruct(v)
		}
		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		if err := t.transformValue(cp); err != nil {
			return reflect.Value{}, err
		}
		return cp, nil
	}

	if v.Kind() == reflect.Array {
		if v.CanSet() {
			return reflect.Value{}, t.transformValue(v)
		}
		cp := reflect.New(v.Type()).Elem()
		cp.Set(v)
		if err := t.transformValue(cp); err != nil {
			return reflect.Value{}, err
		}
		return cp, nil
	}

	return reflect.Value{}, t.transformValue(v)
}

func (t *Transformer) transformStruct(val reflect.Value) error {
	if val.Kind() != reflect.Struct {
		return nil
	}
	return t.transformStructDirect(val, t.getTypeInfo(val.Type()))
}

func (t *Transformer) transformStructDirect(val reflect.Value, info *typeInfo) error {
	if !info.has {
		return nil
	}
	for i := range info.fields {
		if err := info.fields[i].execute(val); err != nil {
			rt := val.Type()
			name := rt.Name()
			if name == "" {
				name = rt.String() // anonymous struct type
			}
			sf := rt.Field(info.fields[i].index)
			return &FieldError{
				Type:  name,
				Field: sf.Name,
				Key:   info.fields[i].key,
				Err:   err,
			}
		}
	}
	return nil
}

func (t *Transformer) transformSlice(val reflect.Value) error {
	elemType := val.Type().Elem()
	elemInfo := t.getTypeInfo(elemType)
	if !elemInfo.has {
		return nil
	}
	for j := 0; j < val.Len(); j++ {
		elem := val.Index(j)
		replacement, err := t.transformElem(elem, elemInfo)
		if err != nil {
			return err
		}
		if replacement.IsValid() {
			elem.Set(replacement)
		}
	}
	return nil
}

func (t *Transformer) transformMap(val reflect.Value) error {
	elemType := val.Type().Elem()
	elemInfo := t.getTypeInfo(elemType)
	if !elemInfo.has {
		return nil
	}

	// Fast path: struct or array values — reuse one copy buffer.
	if elemType.Kind() == reflect.Struct || elemType.Kind() == reflect.Array {
		tmp := reflect.New(elemType).Elem()
		iter := val.MapRange()
		for iter.Next() {
			tmp.Set(iter.Value())
			var err error
			if elemType.Kind() == reflect.Struct {
				err = t.transformStructDirect(tmp, elemInfo)
			} else {
				err = t.transformValue(tmp)
			}
			if err != nil {
				return err
			}
			val.SetMapIndex(iter.Key(), tmp)
		}
		return nil
	}

	// General path: interface, pointer, slice, or map values.
	iter := val.MapRange()
	for iter.Next() {
		replacement, err := t.transformElem(iter.Value(), nil)
		if err != nil {
			return err
		}
		if replacement.IsValid() {
			val.SetMapIndex(iter.Key(), replacement)
		}
	}
	return nil
}
