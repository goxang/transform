package transform

import (
	"reflect"
)

// funcKind classifies a registered function.
type funcKind uint8

const (
	funcNone      funcKind = iota
	funcString             // func(string) string
	funcStringErr          // func(string) (string, error)
	funcAny                // func(any) (any, error)
)

// fieldKind describes how a field is processed at runtime.
type fieldKind uint8

const (
	kindSkip            fieldKind = iota // unexported or no-op
	kindStringDirect                     // string with func(string)string
	kindStringErrDirect                  // string with func(string)(string,error)
	kindAnyDirect                        // any field with func(any)(any,error)
	kindAnyInterface                     // interface field with func(any)(any,error)
	kindStruct                           // recurse into nested value struct
	kindPtrStruct                        // deref pointer, recurse into struct
	kindSlice                            // iterate element values, recurse
	kindArray                            // iterate element values, recurse
	kindMap                              // iterate map values, recurse
	kindInterface                        // inspect concrete type at runtime
	kindNoOp                             // no transform function, skip
)

// typeInfo is the per-type execution plan stored in the Transformer's cache.
type typeInfo struct {
	fields   []fieldPlan
	has      bool
	kind     reflect.Kind // Struct, Slice, Array, Map, Interface, or Invalid
	elem     *typeInfo    // for slice/array/map element type
	compiled bool         // true after compileTypeInfo has been called
}

// fieldPlan is the per-field execution plan.
type fieldPlan struct {
	index int       // struct field index
	kind  fieldKind // runtime kind
	key   string    // transformation key (for error messages)

	// Instance-specific transform functions
	strFn  func(string) string
	strErr func(string) (string, error)
	anyFn  func(any) (any, error)

	nested *typeInfo // nested typeInfo (struct/slice/map elements)

	// Precompiled execute closure
	execute func(reflect.Value) error
}

// lookupKey returns the transform key and function kind for a struct field.
func (b *builder) lookupKey(sf reflect.StructField) (key string, kind funcKind) {
	k := sf.Tag.Get(b.t.tag)
	if k == "" {
		return "", funcNone
	}
	return k, b.classifyFunc(k)
}

// classifyFunc resolves key against the registry snapshot captured when the
// builder was created, so the whole plan is built against one consistent set
// of registered functions.
func (b *builder) classifyFunc(key string) funcKind {
	strFn, ok := b.reg[key]
	if ok {
		if strFn.fn != nil {
			return funcString
		}
		return funcStringErr
	}
	if _, ok := b.anyReg[key]; ok {
		return funcAny
	}
	return funcNone
}

// ========== Type cache ==========

// builder carries the per-graph state for one cache miss. graph holds every
// typeInfo built (and currently being built) for the requested type; a type
// already present in graph that is requested again marks a cycle, which is
// resolved after the graph is complete — see finalize.
//
// reg and anyReg are the Transformer's registry maps (references, not
// copies), captured under the read lock when the builder is created. The
// lock orders the capture after any in-flight Register write. Writes cannot
// happen after the capture either: Transform sets the frozen flag before
// any plan build begins, and Register panics once frozen, so the builder
// may read the maps without holding the lock. Every plan in the graph
// therefore sees the same set of functions.
type builder struct {
	t      *Transformer
	reg    map[string]stringTransform
	anyReg map[string]anyTransform
	graph  map[reflect.Type]*typeInfo
	cyclic bool
}

func (t *Transformer) getTypeInfo(rt reflect.Type) *typeInfo {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if cached, ok := t.cache.Load(rt); ok {
		return cached.(*typeInfo)
	}
	t.mu.RLock()
	reg, anyReg := t.registry, t.anyRegistry
	t.mu.RUnlock()
	b := &builder{t: t, reg: reg, anyReg: anyReg, graph: make(map[reflect.Type]*typeInfo)}
	info := b.build(rt)
	b.finalize()
	return info
}

// build returns the typeInfo for rt, building it (and everything reachable
// from it) if it is not already in the cache. A type seen a second time while
// its info is still under construction is a cycle: the in-progress placeholder
// is returned and the cycle is marked for post-processing in finalize.
func (b *builder) build(rt reflect.Type) *typeInfo {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	if cached, ok := b.t.cache.Load(rt); ok {
		return cached.(*typeInfo)
	}
	if info, ok := b.graph[rt]; ok {
		b.cyclic = true
		return info // recursive type placeholder
	}

	info := &typeInfo{kind: rt.Kind()}
	b.graph[rt] = info

	switch rt.Kind() {
	case reflect.Struct:
		b.buildStructInfo(rt, info)
	case reflect.Slice, reflect.Array:
		el := rt.Elem()
		for el.Kind() == reflect.Ptr {
			el = el.Elem()
		}
		info.elem = b.build(el)
		info.has = info.elem.has
	case reflect.Map:
		el := rt.Elem()
		for el.Kind() == reflect.Ptr {
			el = el.Elem()
		}
		info.elem = b.build(el)
		info.has = info.elem.has
	case reflect.Interface:
		// Concrete type unknown at build time; must inspect at runtime.
		info.has = true
	}

	return info
}

// finalize completes the graph. In a cyclic graph some infos were classified
// against placeholders whose `has` flags were not final, so those flags are
// propagated to a fixed point before anything is compiled. Compilation and
// cache publication happen only after the whole graph is consistent, so
// concurrent readers never observe a half-built plan.
func (b *builder) finalize() {
	if b.cyclic {
		for changed := true; changed; {
			changed = false
			for _, info := range b.graph {
				if recomputeHas(info) {
					changed = true
				}
			}
		}
	}
	for _, info := range b.graph {
		b.t.compileTypeInfo(info)
	}
	for rt, info := range b.graph {
		b.t.cache.Store(rt, info)
	}
}

// recomputeHas recalculates info.has from its fields and element type and
// reports whether it changed. Struct has-flags are derived from field kinds;
// container has-flags mirror their element type; interfaces are always
// potentially transformable.
func recomputeHas(info *typeInfo) bool {
	has := info.has
	switch info.kind {
	case reflect.Struct:
		has = false
		for i := range info.fields {
			fp := &info.fields[i]
			switch fp.kind {
			case kindStringDirect, kindStringErrDirect, kindInterface:
				has = true
			}
			if fp.anyFn != nil {
				has = true
			}
			if fp.nested != nil && fp.nested.has {
				has = true
			}
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		has = info.elem != nil && info.elem.has
	}
	if has != info.has {
		info.has = has
		return true
	}
	return false
}

// ========== Struct analysis ==========

func (b *builder) buildStructInfo(rt reflect.Type, info *typeInfo) {
	n := rt.NumField()
	info.fields = make([]fieldPlan, 0, n)

	for i := 0; i < n; i++ {
		sf := rt.Field(i)
		fp := fieldPlan{
			index: i,
		}

		if !sf.IsExported() && !sf.Anonymous {
			fp.kind = kindSkip
			info.fields = append(info.fields, fp)
			continue
		}

		key, fk := b.lookupKey(sf)
		fp.key = key

		b.classifyField(sf.Type, &fp, fk)

		info.fields = append(info.fields, fp)

		// Direct contributions to `has`. For acyclic graphs this is final;
		// for cyclic graphs finalize corrects stale values via recomputeHas.
		switch fp.kind {
		case kindStringDirect, kindStringErrDirect, kindInterface:
			info.has = true
		}
		if fp.anyFn != nil {
			info.has = true
		}
		if fp.nested != nil && fp.nested.has {
			info.has = true
		}
	}
}

func (b *builder) classifyField(ft reflect.Type, fp *fieldPlan, fk funcKind) {
	baseType := ft
	for baseType.Kind() == reflect.Ptr {
		baseType = baseType.Elem()
	}

	if baseType.Kind() == reflect.Interface {
		switch fk {
		case funcAny:
			fp.kind = kindAnyInterface
			if fn, ok := b.anyReg[fp.key]; ok {
				fp.anyFn = fn.fn
			}
		case funcString:
			fp.kind = kindInterface
			if fn, ok := b.reg[fp.key]; ok {
				fp.strFn = fn.fn
			}
		case funcStringErr:
			fp.kind = kindInterface
			if fn, ok := b.reg[fp.key]; ok {
				fp.strErr = fn.fnErr
			}
		default:
			fp.kind = kindInterface
		}
		return
	}

	isPtr := ft.Kind() == reflect.Ptr

	if baseType.Kind() == reflect.String {
		switch fk {
		case funcString:
			fp.kind = kindStringDirect
			if fn, ok := b.reg[fp.key]; ok {
				fp.strFn = fn.fn
			}
		case funcStringErr:
			fp.kind = kindStringErrDirect
			if fn, ok := b.reg[fp.key]; ok {
				fp.strErr = fn.fnErr
			}
		case funcAny:
			fp.kind = kindAnyDirect
			if fn, ok := b.anyReg[fp.key]; ok {
				fp.anyFn = fn.fn
			}
		default:
			fp.kind = kindNoOp
		}
		return
	}

	// Compound types (struct/slice/array/map) are always traversed, whether or
	// not they arrive behind a pointer. A funcAny transform registered on the
	// field is applied on top of the traversal. funcString/funcStringErr
	// transforms are inapplicable to compounds and are ignored while traversal
	// continues into inner tagged fields.
	if fk == funcAny {
		if fn, ok := b.anyReg[fp.key]; ok {
			fp.anyFn = fn.fn
		}
	}

	switch baseType.Kind() {
	case reflect.Struct:
		if isPtr {
			fp.kind = kindPtrStruct
		} else {
			fp.kind = kindStruct
		}
		fp.nested = b.build(baseType)

	case reflect.Slice:
		fp.kind = kindSlice
		el := baseType.Elem()
		for el.Kind() == reflect.Ptr {
			el = el.Elem()
		}
		fp.nested = b.build(el)

	case reflect.Array:
		fp.kind = kindArray
		el := baseType.Elem()
		for el.Kind() == reflect.Ptr {
			el = el.Elem()
		}
		fp.nested = b.build(el)

	case reflect.Map:
		fp.kind = kindMap
		el := baseType.Elem()
		for el.Kind() == reflect.Ptr {
			el = el.Elem()
		}
		fp.nested = b.build(el)

	default:
		// Scalar non-string field (int, bool, ...): only funcAny applies.
		if fk == funcAny {
			fp.kind = kindAnyDirect
		} else {
			fp.kind = kindNoOp
		}
	}
}
