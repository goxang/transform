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
	kindSkip            fieldKind = iota // unexported, untagged, or otherwise no work
	kindStringDirect                     // string with func(string)string
	kindStringErrDirect                  // string with func(string)(string,error)
	kindStringContainer                  // slice/array/map bottoming out in strings
	kindAnyDirect                        // any field with func(any)(any,error)
	kindAnyInterface                     // interface field with func(any)(any,error)
	kindStruct                           // recurse into nested value struct
	kindPtrStruct                        // deref pointer, recurse into struct
	kindSlice                            // iterate element values, recurse
	kindArray                            // iterate element values, recurse
	kindMap                              // iterate map values, recurse
	kindInterface                        // inspect concrete type at runtime
	kindStrictErr                        // strict mode: report a bad tag on every call
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
	err    error     // strict-mode tag diagnosis, reported on every call

	// Precompiled execute closure
	execute func(reflect.Value, int) error
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

// lookupKey returns the transform key and function kind for a struct field.
func (b *builder) lookupKey(sf *reflect.StructField) (key string, kind funcKind) {
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

func (t *Transformer) getTypeInfo(rt reflect.Type) *typeInfo {
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if cached, ok := t.cache.Load(rt); ok {
		return cached.(*typeInfo)
	}
	t.mu.RLock()
	reg, anyReg := t.registry, t.anyRegistry
	t.mu.RUnlock()
	b := &builder{t: t, reg: reg, anyReg: anyReg, graph: make(map[reflect.Type]*typeInfo)}
	b.build(rt)
	b.finalize()
	// finalize publishes with LoadOrStore, so a plan another goroutine built
	// for the same type concurrently is the one that wins. Read the cache
	// back rather than returning the local graph node, so every caller ends
	// up on the same plan instance.
	cached, _ := t.cache.Load(rt)
	return cached.(*typeInfo)
}

// build returns the typeInfo for rt, building it (and everything reachable
// from it) if it is not already in the cache. A type seen a second time while
// its info is still under construction is a cycle: the in-progress placeholder
// is returned and the cycle is marked for post-processing in finalize.
func (b *builder) build(rt reflect.Type) *typeInfo {
	for rt.Kind() == reflect.Pointer {
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
	case reflect.Slice, reflect.Array, reflect.Map:
		info.elem = b.build(derefType(rt.Elem()))
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
		b.t.cache.LoadOrStore(rt, info)
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
			if fieldDoesWork(&info.fields[i]) {
				has = true
				break
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

// fieldDoesWork reports whether a field plan can change anything (or, in
// strict mode, report anything) at runtime. It is the single definition of
// "this struct is worth walking"; both the initial build and the cyclic-graph
// fixed point use it, so the two can never drift apart.
func fieldDoesWork(fp *fieldPlan) bool {
	switch fp.kind {
	case kindStringDirect, kindStringErrDirect, kindStringContainer, kindInterface, kindStrictErr:
		return true
	}
	if fp.anyFn != nil {
		return true
	}
	return fp.nested != nil && fp.nested.has
}

// ========== Struct analysis ==========

func (b *builder) buildStructInfo(rt reflect.Type, info *typeInfo) {
	n := rt.NumField()
	info.fields = make([]fieldPlan, 0, n)

	for i := 0; i < n; i++ {
		sf := rt.Field(i)
		fp := fieldPlan{index: i}

		if !sf.IsExported() && !sf.Anonymous {
			// An unexported field can never be set. A tag on one is always a
			// mistake, so strict mode says so instead of ignoring it.
			if key := sf.Tag.Get(b.t.tag); key != "" && b.t.strict {
				fp.key, fp.kind, fp.err = key, kindStrictErr, ErrUnusableKey
				info.has = true
			} else {
				fp.kind = kindSkip
			}
			info.fields = append(info.fields, fp)
			continue
		}

		key, fk := b.lookupKey(&sf)
		fp.key = key

		b.classifyField(sf.Type, &fp, fk)

		if b.t.strict && key != "" && fp.kind != kindStrictErr {
			if err := strictDiagnosis(&fp, fk); err != nil {
				fp.kind, fp.err = kindStrictErr, err
			}
		}

		info.fields = append(info.fields, fp)

		// Direct contribution to `has`. For acyclic graphs this is final;
		// for cyclic graphs finalize corrects stale values via recomputeHas.
		if fieldDoesWork(&fp) {
			info.has = true
		}
	}
}

// strictDiagnosis reports why a non-empty tag key produced no transformation,
// or nil if the key is doing its job. Only consulted in strict mode.
func strictDiagnosis(fp *fieldPlan, fk funcKind) error {
	if fk == funcNone {
		return ErrUnknownKey
	}
	// A registered key that reached a field it cannot act on: a string
	// function on a compound or scalar field. Traversal still happens, but
	// the key itself does nothing, which is what strict mode is there to
	// catch.
	switch fp.kind {
	case kindStringDirect, kindStringErrDirect, kindStringContainer,
		kindAnyDirect, kindAnyInterface, kindInterface:
		return nil
	}
	if fp.anyFn != nil {
		return nil
	}
	return ErrUnusableKey
}

// derefType unwraps pointer indirection from a type.
func derefType(rt reflect.Type) reflect.Type {
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	return rt
}

// maxTypeUnwrap bounds the element walk in hasStringLeaf. Self-referential
// container types (`type S []S`) would otherwise loop forever; no real type
// nests containers anywhere near this deep.
const maxTypeUnwrap = 100

// hasStringLeaf reports whether unwrapping pointers and container element
// types from rt bottoms out in a string kind — `[]string`, `map[K]*string`,
// `[][]MyString`, and so on. Such a container carries no struct fields, so a
// string transform tagged on it can be applied to its elements directly.
func hasStringLeaf(rt reflect.Type) bool {
	for i := 0; i < maxTypeUnwrap; i++ {
		switch rt.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			rt = rt.Elem()
		case reflect.String:
			return true
		default:
			return false
		}
	}
	return false
}

func (b *builder) classifyField(ft reflect.Type, fp *fieldPlan, fk funcKind) {
	baseType := derefType(ft)

	if baseType.Kind() == reflect.Interface {
		switch fk {
		case funcAny:
			fp.kind = kindAnyInterface
			fp.anyFn = b.anyReg[fp.key].fn
		case funcString:
			fp.kind = kindInterface
			fp.strFn = b.reg[fp.key].fn
		case funcStringErr:
			fp.kind = kindInterface
			fp.strErr = b.reg[fp.key].fnErr
		default:
			fp.kind = kindInterface
		}
		return
	}

	if baseType.Kind() == reflect.String {
		switch fk {
		case funcString:
			fp.kind = kindStringDirect
			fp.strFn = b.reg[fp.key].fn
		case funcStringErr:
			fp.kind = kindStringErrDirect
			fp.strErr = b.reg[fp.key].fnErr
		case funcAny:
			fp.kind = kindAnyDirect
			fp.anyFn = b.anyReg[fp.key].fn
		default:
			fp.kind = kindSkip
		}
		return
	}

	// A string transform on a container whose elements bottom out in strings
	// is applied to those elements. Such a container has no struct fields to
	// traverse, so this replaces traversal rather than adding to it.
	if (fk == funcString || fk == funcStringErr) && hasStringLeaf(baseType) {
		switch baseType.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			fp.kind = kindStringContainer
			fp.strFn = b.reg[fp.key].fn
			fp.strErr = b.reg[fp.key].fnErr
			return
		}
	}

	// Compound types (struct/slice/array/map) are always traversed, whether or
	// not they arrive behind a pointer. A funcAny transform registered on the
	// field is applied on top of the traversal. funcString/funcStringErr
	// transforms are inapplicable to compounds of non-strings and are ignored
	// while traversal continues into inner tagged fields.
	if fk == funcAny {
		fp.anyFn = b.anyReg[fp.key].fn
	}

	switch baseType.Kind() {
	case reflect.Struct:
		if ft.Kind() == reflect.Pointer {
			fp.kind = kindPtrStruct
		} else {
			fp.kind = kindStruct
		}
		fp.nested = b.build(baseType)

	case reflect.Slice:
		fp.kind = kindSlice
		fp.nested = b.build(derefType(baseType.Elem()))

	case reflect.Array:
		fp.kind = kindArray
		fp.nested = b.build(derefType(baseType.Elem()))

	case reflect.Map:
		fp.kind = kindMap
		fp.nested = b.build(derefType(baseType.Elem()))

	default:
		// Scalar non-string field (int, bool, ...): only funcAny applies.
		if fk == funcAny {
			fp.kind = kindAnyDirect
		} else {
			fp.kind = kindSkip
		}
	}
}
