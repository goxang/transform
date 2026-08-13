package transform_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/goxang/transform"
)

// ========== Test types ==========

type flat struct {
	Name   string `json:"name"`
	Email  string `json:"email" transform:"upper"`
	Phone  string `json:"phone"`
	Secret string `json:"secret" transform:"mask"`
}

type nested struct {
	Inner flat
	ID    string `json:"id"`
}

type deepL3 struct {
	Secret string `json:"secret" transform:"upper"`
}
type deepL2 struct{ L3 deepL3 }
type deepL1 struct{ L1 deepL2 }

type withSlice struct {
	Items []flat
}

type withMap struct {
	Items map[string]flat
}

type withPtrMap struct {
	Items map[string]*flat
}

type withPtr struct {
	Item *flat
}

type withIface struct {
	Item interface{}
}

type noTransforms struct {
	A string `json:"a"`
	B int    `json:"b"`
}

type embeddedFlat struct {
	flat
	Tag string `json:"tag"`
}

type mixedSlice struct {
	PtrSlice  []*flat          `json:"ptr_slice"`
	MapStrPtr map[string]*flat `json:"map_str_ptr"`
}

type StampInner struct {
	Email  string `json:"email" transform:"upper"`
	Secret string `json:"secret" transform:"mask"`
}

type StampOuter struct {
	StampInner `transform:"stamp"`
	ID         string `json:"id"`
}

// Types for embedded interface field tests. Methods are needed so the
// interfaces can only be satisfied by these concrete types.
type hiddenIface interface{ M() }

type hiddenImpl struct {
	Name string `transform:"upper"`
}

func (hiddenImpl) M() {}

type hiddenStr string

func (hiddenStr) M() {}

type PublicIface interface{ M() }

type publicIfaceImpl struct {
	Name string `transform:"upper"`
}

func (publicIfaceImpl) M() {}

// ========== Helper ==========

func newTransformer() *transform.Transformer {
	t := transform.New()
	t.RegisterString("upper", strings.ToUpper)
	t.RegisterString("mask", func(s string) string {
		if len(s) <= 4 {
			return "****"
		}
		return s[:2] + "****" + s[len(s)-2:]
	})
	t.RegisterStringErr("upper_err", func(s string) (string, error) {
		return strings.ToUpper(s), nil
	})
	t.RegisterStringErr("fail_err", func(_ string) (string, error) {
		return "", errors.New("intentional failure")
	})
	t.RegisterAny("prepend", func(v any) (any, error) {
		return "x_" + fmt.Sprint(v), nil
	})
	return t
}

// ========== Basic tests ==========

func TestTransformFlat(t *testing.T) {
	tx := newTransformer()
	v := flat{Name: "alice", Email: "alice@example.com", Secret: "abcdefgh"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Email != "ALICE@EXAMPLE.COM" {
		t.Errorf("Email: got %q, want %q", v.Email, "ALICE@EXAMPLE.COM")
	}
	if v.Secret != "ab****gh" {
		t.Errorf("Secret: got %q, want %q", v.Secret, "ab****gh")
	}
	if v.Name != "alice" {
		t.Errorf("Name: got %q, want unchanged", v.Name)
	}
}

func TestTransformNested(t *testing.T) {
	tx := newTransformer()
	v := nested{Inner: flat{Email: "test@test.com", Secret: "12345678"}, ID: "keep"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Inner.Email != "TEST@TEST.COM" {
		t.Errorf("nested Email: got %q", v.Inner.Email)
	}
	if v.Inner.Secret != "12****78" {
		t.Errorf("nested Secret: got %q", v.Inner.Secret)
	}
	if v.ID != "keep" {
		t.Errorf("ID changed: got %q", v.ID)
	}
}

func TestTransformDeepNested(t *testing.T) {
	tx := newTransformer()
	v := deepL1{L1: deepL2{L3: deepL3{Secret: "hello"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.L1.L3.Secret != "HELLO" {
		t.Errorf("deep Secret: got %q", v.L1.L3.Secret)
	}
}

func TestTransformSlice(t *testing.T) {
	tx := newTransformer()
	v := withSlice{Items: []flat{
		{Email: "a@a.com", Secret: "12345678"},
		{Email: "b@b.com", Secret: "87654321"},
	}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Items[0].Email != "A@A.COM" {
		t.Errorf("slice[0] Email: got %q", v.Items[0].Email)
	}
	if v.Items[1].Secret != "87****21" {
		t.Errorf("slice[1] Secret: got %q", v.Items[1].Secret)
	}
}

func TestTransformPtrSlice(t *testing.T) {
	tx := newTransformer()
	a := flat{Email: "a@a.com", Secret: "12345678"}
	b := flat{Email: "b@b.com", Secret: "87654321"}
	v := struct {
		Items []*flat
	}{Items: []*flat{&a, &b}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if a.Email != "A@A.COM" {
		t.Errorf("ptr slice[0] Email: got %q", a.Email)
	}
	if b.Secret != "87****21" {
		t.Errorf("ptr slice[1] Secret: got %q", b.Secret)
	}
}

func TestTransformMap(t *testing.T) {
	tx := newTransformer()
	v := withMap{Items: map[string]flat{
		"a": {Email: "a@a.com", Secret: "12345678"},
		"b": {Email: "b@b.com", Secret: "87654321"},
	}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Items["a"].Email != "A@A.COM" {
		t.Errorf("map[a] Email: got %q", v.Items["a"].Email)
	}
	if v.Items["b"].Secret != "87****21" {
		t.Errorf("map[b] Secret: got %q", v.Items["b"].Secret)
	}
}

func TestTransformMapPtr(t *testing.T) {
	tx := newTransformer()
	a := flat{Email: "a@a.com", Secret: "12345678"}
	b := flat{Email: "b@b.com", Secret: "87654321"}
	v := withPtrMap{Items: map[string]*flat{"a": &a, "b": &b}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if a.Email != "A@A.COM" {
		t.Errorf("map ptr a: got %q", a.Email)
	}
	if b.Secret != "87****21" {
		t.Errorf("map ptr b: got %q", b.Secret)
	}
}

func TestTransformPointerField(t *testing.T) {
	tx := newTransformer()
	inner := flat{Email: "test@test.com", Secret: "12345678"}
	v := withPtr{Item: &inner}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if inner.Email != "TEST@TEST.COM" {
		t.Errorf("ptr field Email: got %q", inner.Email)
	}
}

func TestTransformPointerFieldNil(t *testing.T) {
	tx := newTransformer()
	v := withPtr{Item: nil}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
}

func TestTransformNoTransforms(t *testing.T) {
	tx := newTransformer()
	v := noTransforms{A: "hello", B: 42}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.A != "hello" || v.B != 42 {
		t.Error("no-transform struct changed")
	}
}

func TestTransformEmbedded(t *testing.T) {
	tx := newTransformer()
	v := embeddedFlat{
		flat: flat{Email: "test@test.com", Secret: "12345678"},
		Tag:  "keep",
	}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Email != "TEST@TEST.COM" {
		t.Errorf("embedded Email: got %q", v.Email)
	}
	if v.Tag != "keep" {
		t.Errorf("Tag changed: %q", v.Tag)
	}
}

func TestTransformInterface(t *testing.T) {
	tx := newTransformer()
	inner := flat{Email: "test@test.com", Secret: "12345678"}
	v := withIface{Item: inner}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	result, ok := v.Item.(flat)
	if !ok {
		t.Fatal("interface type changed")
	}
	if result.Email != "TEST@TEST.COM" {
		t.Errorf("interface Email: got %q", result.Email)
	}
}

func TestTransformPassByValue(t *testing.T) {
	tx := newTransformer()
	v := flat{Email: "test@test.com"}
	err := tx.Transform(v)
	if err == nil {
		t.Fatal("expected error for non-pointer src")
	}
}

func TestTransformNil(t *testing.T) {
	tx := newTransformer()
	var v *flat
	err := tx.Transform(v)
	if err == nil {
		t.Fatal("expected error for nil src")
	}
}

// ========== Error handling ==========

func TestTransformStringErrSuccess(t *testing.T) {
	tx := transform.New()
	tx.RegisterStringErr("upper_err", func(s string) (string, error) {
		return strings.ToUpper(s), nil
	})
	type obj struct {
		Name string `transform:"upper_err"`
	}
	v := obj{Name: "hello"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Name != "HELLO" {
		t.Errorf("got %q", v.Name)
	}
}

func TestTransformStringErrFailure(t *testing.T) {
	tx := transform.New()
	tx.RegisterStringErr("fail", func(_ string) (string, error) {
		return "", errors.New("intentional failure")
	})
	type obj struct {
		Name string `transform:"fail"`
	}
	v := obj{Name: "hello"}
	err := tx.Transform(&v)
	if err == nil {
		t.Fatal("expected error")
	}
	var fe *transform.FieldError
	if !errors.As(err, &fe) {
		t.Errorf("error should be *FieldError, got %T: %v", err, err)
	}
}

func TestTransformAny(t *testing.T) {
	tx := transform.New()
	tx.RegisterAny("prepend", func(v any) (any, error) {
		return "x_" + fmt.Sprint(v), nil
	})
	type obj struct {
		Name string `transform:"prepend"`
	}
	v := obj{Name: "hello"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Name != "x_hello" {
		t.Errorf("got %q", v.Name)
	}
}

// ========== Custom tag ==========

func TestCustomTag(t *testing.T) {
	tx := transform.New(transform.WithTag("xform"))
	tx.RegisterString("upper", strings.ToUpper)
	type obj struct {
		Name string `xform:"upper"`
	}
	v := obj{Name: "alice"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Name != "ALICE" {
		t.Errorf("got %q", v.Name)
	}
}

// ========== Mixed collections ==========

func TestMixedSliceAndMap(t *testing.T) {
	tx := newTransformer()
	a := flat{Email: "a@a.com", Secret: "1234"}
	b := flat{Email: "b@b.com", Secret: "5678"}
	c := flat{Email: "c@c.com", Secret: "abcd"}
	v := mixedSlice{
		PtrSlice:  []*flat{&a, &b},
		MapStrPtr: map[string]*flat{"x": &c},
	}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if a.Email != "A@A.COM" {
		t.Errorf("PtrSlice[0] Email: %q", a.Email)
	}
	if b.Secret != "****" {
		t.Errorf("PtrSlice[1] Secret: %q", b.Secret)
	}
	if c.Email != "C@C.COM" {
		t.Errorf("MapStrPtr[x] Email: %q", c.Email)
	}
}

func TestSliceOfMaps(t *testing.T) {
	tx := newTransformer()
	v := struct {
		Items []map[string]flat
	}{
		Items: []map[string]flat{
			{"a": {Email: "a@a.com", Secret: "12345678"}},
		},
	}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Items[0]["a"].Email != "A@A.COM" {
		t.Errorf("slice[0]map[a] Email: %q", v.Items[0]["a"].Email)
	}
}

// ========== Edge cases ==========

func TestAllFieldsEmpty(t *testing.T) {
	tx := newTransformer()
	v := flat{}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
}

func TestUnchangedFields(t *testing.T) {
	tx := newTransformer()
	v := flat{Name: "alice", Phone: "12345"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Name != "alice" {
		t.Errorf("Name changed: %q", v.Name)
	}
	if v.Phone != "12345" {
		t.Errorf("Phone changed: %q", v.Phone)
	}
}

func TestMultipleTransformsSameStruct(t *testing.T) {
	tx := newTransformer()
	v1 := flat{Email: "a@a.com", Secret: "12345678"}
	v2 := flat{Email: "b@b.com", Secret: "87654321"}
	if err := tx.Transform(&v1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Transform(&v2); err != nil {
		t.Fatal(err)
	}
	if v1.Email != "A@A.COM" {
		t.Errorf("v1 Email: %q", v1.Email)
	}
	if v2.Email != "B@B.COM" {
		t.Errorf("v2 Email: %q", v2.Email)
	}
}

func TestRecursiveType(t *testing.T) {
	tx := newTransformer()
	type tree struct {
		Value string `transform:"upper"`
		Left  *tree
		Right *tree
	}
	root := &tree{
		Value: "root",
		Left:  &tree{Value: "left", Left: &tree{Value: "deep"}},
		Right: &tree{Value: "right"},
	}
	if err := tx.Transform(root); err != nil {
		t.Fatal(err)
	}
	if root.Value != "ROOT" {
		t.Errorf("root: %q", root.Value)
	}
	if root.Left.Value != "LEFT" {
		t.Errorf("left: %q", root.Left.Value)
	}
	if root.Left.Left.Value != "DEEP" {
		t.Errorf("deep: %q", root.Left.Left.Value)
	}
	if root.Right.Value != "RIGHT" {
		t.Errorf("right: %q", root.Right.Value)
	}
}

// ========== Concurrent tests ==========

func TestConcurrentTransform(t *testing.T) {
	tx := newTransformer()
	// Warm cache
	v := flat{Email: "x@x.com", Secret: "12345678"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Email != "X@X.COM" {
		t.Errorf("warm transform: got %q, want X@X.COM", v.Email)
	}

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			v := flat{Email: "test@test.com", Secret: "abcdefgh"}
			if err := tx.Transform(&v); err != nil {
				t.Error(err)
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== Recursive types ==========

type testNodeA struct {
	Name string `transform:"upper"`
	B    *testNodeB
}

type testNodeB struct {
	A *testNodeA
}

func TestMutuallyRecursiveTypes(t *testing.T) {
	// nodeA → nodeB → nodeA: nodeB has no directly-tagged fields, so the
	// traversal plan for it depends on nodeA's plan, which is still being
	// built when nodeB is analyzed. Regression test for `has` flags being
	// computed before the whole type graph was finalized.
	tx := newTransformer()

	inner := &testNodeA{Name: "inner"}
	root := &testNodeA{Name: "root", B: &testNodeB{A: inner}}
	if err := tx.Transform(root); err != nil {
		t.Fatal(err)
	}
	if root.Name != "ROOT" {
		t.Errorf("root: got %q, want ROOT", root.Name)
	}
	if root.B.A.Name != "INNER" {
		t.Errorf("mutually recursive subtree: got %q, want INNER", root.B.A.Name)
	}
}

func TestSelfRecursiveSliceType(t *testing.T) {
	tx := newTransformer()
	type tree struct {
		Value    string `transform:"upper"`
		Children []tree
	}
	root := tree{
		Value:    "root",
		Children: []tree{{Value: "leaf"}},
	}
	if err := tx.Transform(&root); err != nil {
		t.Fatal(err)
	}
	if root.Value != "ROOT" {
		t.Errorf("root: got %q, want ROOT", root.Value)
	}
	if root.Children[0].Value != "LEAF" {
		t.Errorf("slice child: got %q, want LEAF", root.Children[0].Value)
	}
}

func TestRecursiveTypeNoTransforms(t *testing.T) {
	tx := newTransformer()
	type empty struct {
		Next *empty
	}
	v := &empty{Next: &empty{}}
	if err := tx.Transform(v); err != nil {
		t.Fatal(err)
	}
}

// ========== Pointer-to-interface fields ==========

func TestTransformPtrToInterface(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item *any
	}
	item := any(flat{Email: "a@a.com", Secret: "12345678"})
	v := obj{Item: &item}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	result, ok := (*v.Item).(flat)
	if !ok {
		t.Fatalf("interface type changed: %T", *v.Item)
	}
	if result.Email != "A@A.COM" {
		t.Errorf("ptr-to-interface Email: got %q", result.Email)
	}
}

func TestTransformPtrToInterfaceAnyKey(t *testing.T) {
	tx := transform.New()
	tx.RegisterAny("double", func(v any) (any, error) { return v.(int) * 2, nil })
	type obj struct {
		Item *any `transform:"double"`
	}
	item := any(21)
	v := obj{Item: &item}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if *v.Item != 42 {
		t.Errorf("ptr-to-interface any: got %v, want 42", *v.Item)
	}
}

// ========== Freeze-before-validation ==========

func TestTransformInvalidInputNotFrozen(t *testing.T) {
	tx := transform.New()

	var x int
	if err := tx.Transform(&x); err == nil {
		t.Fatal("expected ErrInvalidSrc for &int")
	}

	// Must NOT panic — transformer is still mutable.
	tx.RegisterString("upper", strings.ToUpper)
}

func TestTransformInvalidDoublePointerNotFrozen(t *testing.T) {
	tx := transform.New()

	x := 1
	px := &x
	if err := tx.Transform(&px); err == nil {
		t.Fatal("expected ErrInvalidSrc for &&int")
	}

	// Must NOT panic.
	tx.RegisterString("upper", strings.ToUpper)
}

func TestTransformInvalidNilNotFrozen(t *testing.T) {
	tx := transform.New()

	if err := tx.Transform(nil); err == nil {
		t.Fatal("expected ErrInvalidSrc for nil")
	}

	// Must NOT panic.
	tx.RegisterString("upper", strings.ToUpper)
}

// ========== Transform valid types ==========

func TestTransformSliceRoot(t *testing.T) {
	tx := newTransformer()
	type item struct {
		Name string `transform:"upper"`
	}
	v := []item{{Name: "alice"}, {Name: "bob"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v[0].Name != "ALICE" || v[1].Name != "BOB" {
		t.Errorf("slice root: got %q, %q", v[0].Name, v[1].Name)
	}
}

func TestTransformMapRoot(t *testing.T) {
	tx := newTransformer()
	type item struct {
		Name string `transform:"upper"`
	}
	v := map[string]item{"a": {Name: "alice"}, "b": {Name: "bob"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v["a"].Name != "ALICE" || v["b"].Name != "BOB" {
		t.Errorf("map root: got %q, %q", v["a"].Name, v["b"].Name)
	}
}

func TestTransformDoublePointerStruct(t *testing.T) {
	tx := newTransformer()
	v := flat{Email: "test@test.com", Secret: "12345678"}
	pv := &v
	if err := tx.Transform(&pv); err != nil {
		t.Fatal(err)
	}
	if v.Email != "TEST@TEST.COM" {
		t.Errorf("**struct Email: got %q", v.Email)
	}
}

// ========== FieldError ==========

func TestFieldError(t *testing.T) {
	inner := errors.New("inner error")
	fe := &transform.FieldError{
		Type:  "MyStruct",
		Field: "MyField",
		Key:   "upper",
		Err:   inner,
	}
	errStr := fe.Error()
	if errStr == "" {
		t.Fatal("Error() returned empty string")
	}
	if !strings.Contains(errStr, "MyStruct") {
		t.Errorf("Error missing type: %s", errStr)
	}
	if !strings.Contains(errStr, "MyField") {
		t.Errorf("Error missing field: %s", errStr)
	}
	if !strings.Contains(errStr, "upper") {
		t.Errorf("Error missing key: %s", errStr)
	}
	if !errors.Is(fe, inner) {
		t.Error("errors.Is should find inner error via Unwrap")
	}
	var fe2 *transform.FieldError
	if !errors.As(fe, &fe2) {
		t.Error("errors.As should match *FieldError")
	}
}

func TestFieldErrorNoKey(t *testing.T) {
	inner := errors.New("boom")
	fe := &transform.FieldError{
		Type:  "MyStruct",
		Field: "MyField",
		Err:   inner,
	}
	errStr := fe.Error()
	if !strings.Contains(errStr, "boom") {
		t.Errorf("Error missing underlying message: %s", errStr)
	}
	// key should not appear
	if strings.Contains(errStr, "(key") {
		t.Errorf("Error should not mention key when empty: %s", errStr)
	}
}

// ========== Freeze-panic on register after use ==========

func TestRegisterAfterTransformPanics(t *testing.T) {
	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)
	type obj struct {
		Name string `transform:"upper"`
	}
	_ = tx.Transform(&obj{Name: "x"})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on RegisterString after Transform")
		}
	}()
	tx.RegisterString("lower", strings.ToLower)
}

func TestRegisterStringErrAfterTransformPanics(t *testing.T) {
	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)
	type obj struct {
		Name string `transform:"upper"`
	}
	_ = tx.Transform(&obj{Name: "x"})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterStringErr("lower", func(s string) (string, error) { return s, nil })
}

func TestRegisterAnyAfterTransformPanics(t *testing.T) {
	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)
	type obj struct {
		Name string `transform:"upper"`
	}
	_ = tx.Transform(&obj{Name: "x"})

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterAny("prepend", func(v any) (any, error) { return v, nil })
}

// ========== String field edge cases ==========

func TestTransformShortStringMask(t *testing.T) {
	tx := transform.New()
	tx.RegisterString("mask", func(s string) string {
		if len(s) <= 4 {
			return "****"
		}
		return s[:2] + "****" + s[len(s)-2:]
	})
	type obj struct {
		Secret string `transform:"mask"`
	}
	v := obj{Secret: "ab"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Secret != "****" {
		t.Errorf("short string: got %q", v.Secret)
	}
}

func TestTransformNilInterface(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item interface{}
	}
	v := obj{Item: nil}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
}

// ========== Concurrent first-use cache population ==========

func TestConcurrentColdCacheSameType(t *testing.T) {
	// Verify multiple goroutines building the same type's cache
	// simultaneously does not race, deadlock, or produce wrong results.

	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)

	type item struct {
		Name string `transform:"upper"`
	}

	const N = 100
	var wg sync.WaitGroup
	wg.Add(N)

	for i := 0; i < N; i++ {
		go func(id int) {
			defer wg.Done()
			v := &item{Name: fmt.Sprintf("g%d", id)}
			if err := tx.Transform(v); err != nil {
				t.Errorf("goroutine %d: %v", id, err)
			} else if v.Name != fmt.Sprintf("G%d", id) {
				t.Errorf("goroutine %d: got %q, want %q", id, v.Name, fmt.Sprintf("G%d", id))
			}
		}(i)
	}
	wg.Wait()
}

// ========== Concurrent registration (regression) ==========

func TestConcurrentRegisterDuringTransform(t *testing.T) {
	// Registering while another goroutine runs Transform must not race on
	// the registry maps. The freeze check makes a Register call panic once
	// the first Transform has run, so panics are recovered here; the race
	// detector verifies memory safety. Each Transform value cycles a
	// distinct type to force cache misses, which is when the registries
	// are read.
	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)

	vals := []any{
		&flat{Email: "x@x.com"},
		&nested{Inner: flat{Email: "x@x.com"}},
		&deepL1{},
		&withSlice{Items: []flat{{Email: "x@x.com"}}},
		&withMap{Items: map[string]flat{"a": {Email: "x@x.com"}}},
		&withPtrMap{},
		&withPtr{Item: &flat{Email: "x@x.com"}},
		&withIface{Item: flat{Email: "x@x.com"}},
		&mixedSlice{},
		&embeddedFlat{flat: flat{Email: "x@x.com"}},
		&StampOuter{StampInner: StampInner{Email: "x@x.com"}},
		&noTransforms{},
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			_ = tx.Transform(vals[i%len(vals)])
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 400; i++ {
			func() {
				defer func() { _ = recover() }()
				tx.RegisterString(fmt.Sprintf("k%d", i%8), strings.ToUpper)
			}()
		}
	}()
	wg.Wait()
}

// ========== Edge cases ==========

type namedString string

func TestNamedStringType(t *testing.T) {
	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)

	type obj struct {
		Label namedString `transform:"upper"`
	}
	v := obj{Label: "hello"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	// namedString is string-based; RegisterString applies to
	// reflect.String kind, so this should work.
	if v.Label != "HELLO" {
		t.Errorf("named string: got %q", v.Label)
	}
}

func TestInterfaceHoldingPointer(t *testing.T) {
	tx := newTransformer()

	type inner struct {
		Secret string `transform:"upper"`
	}
	type obj struct {
		Item interface{}
	}
	v := obj{Item: &inner{Secret: "hello"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	innerPtr, ok := v.Item.(*inner)
	if !ok {
		t.Fatal("interface type changed")
	}
	if innerPtr.Secret != "HELLO" {
		t.Errorf("interface ptr: got %q", innerPtr.Secret)
	}
}

func TestTransformEmptySlice(t *testing.T) {
	tx := newTransformer()
	type item struct {
		Name string `transform:"upper"`
	}
	v := []item{}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
}

func TestTransformEmptyMap(t *testing.T) {
	tx := newTransformer()
	type item struct {
		Name string `transform:"upper"`
	}
	v := map[string]item{}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
}

func TestTransformNilSliceField(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items []flat `json:"items"`
	}
	v := obj{Items: nil}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
}

func TestTransformSliceOfPtrsWithNilElement(t *testing.T) {
	tx := newTransformer()
	a := flat{Email: "a@a.com", Secret: "12345678"}
	type obj struct {
		Items []*flat
	}
	v := obj{Items: []*flat{&a, nil, &flat{Email: "b@b.com"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if a.Email != "A@A.COM" {
		t.Errorf("ptr slice[0]: %q", a.Email)
	}
}

func TestRegisterStringEmptyKeyPanics(t *testing.T) {
	tx := transform.New()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterString("", strings.ToUpper)
}

func TestRegisterStringNilFnPanics(t *testing.T) {
	tx := transform.New()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterString("key", nil)
}

func TestRegisterStringErrEmptyKeyPanics(t *testing.T) {
	tx := transform.New()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterStringErr("", func(s string) (string, error) { return s, nil })
}

func TestRegisterStringErrNilFnPanics(t *testing.T) {
	tx := transform.New()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterStringErr("key", nil)
}

func TestRegisterAnyEmptyKeyPanics(t *testing.T) {
	tx := transform.New()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterAny("", func(v any) (any, error) { return v, nil })
}

func TestRegisterAnyNilFnPanics(t *testing.T) {
	tx := transform.New()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.RegisterAny("key", nil)
}

// ========== Addressability edge cases (regression) ==========

func TestTransformDoublePointerField(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item **flat
	}
	inner := flat{Email: "test@test.com"}
	ptr := &inner
	v := obj{Item: &ptr}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if inner.Email != "TEST@TEST.COM" {
		t.Errorf("double pointer field: got %q", inner.Email)
	}
}

func TestTransformInterfaceAnyKey(t *testing.T) {
	tx := transform.New()
	tx.RegisterAny("double", func(v any) (any, error) { return v.(int) * 2, nil })
	type obj struct {
		Item interface{} `transform:"double"`
	}
	v := obj{Item: 21}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Item != 42 {
		t.Errorf("interface any: got %v, want 42", v.Item)
	}
}

func TestTransformInterfaceSlice(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item interface{}
	}
	v := obj{Item: []flat{{Email: "a@a.com"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	sl, ok := v.Item.([]flat)
	if !ok {
		t.Fatalf("interface type changed: %T", v.Item)
	}
	if sl[0].Email != "A@A.COM" {
		t.Errorf("interface slice: got %q", sl[0].Email)
	}
}

func TestTransformInterfaceArray(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item interface{}
	}
	v := obj{Item: [2]flat{{Email: "a@a.com"}, {Email: "b@b.com"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	ar, ok := v.Item.([2]flat)
	if !ok {
		t.Fatalf("interface type changed: %T", v.Item)
	}
	if ar[0].Email != "A@A.COM" || ar[1].Email != "B@B.COM" {
		t.Errorf("interface array: got %q %q", ar[0].Email, ar[1].Email)
	}
}

func TestTransformInterfaceMap(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item interface{}
	}
	v := obj{Item: map[string]flat{"a": {Email: "a@a.com"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	m, ok := v.Item.(map[string]flat)
	if !ok {
		t.Fatalf("interface type changed: %T", v.Item)
	}
	if m["a"].Email != "A@A.COM" {
		t.Errorf("interface map: got %q", m["a"].Email)
	}
}

func TestTransformMapInterfaceStruct(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item map[string]interface{}
	}
	v := obj{Item: map[string]interface{}{"a": flat{Email: "a@a.com"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Item["a"].(flat).Email != "A@A.COM" {
		t.Errorf("map[string]any: got %q", v.Item["a"].(flat).Email)
	}
}

func TestTransformSliceOfInterface(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items []interface{}
	}
	v := obj{Items: []interface{}{flat{Email: "a@a.com"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Items[0].(flat).Email != "A@A.COM" {
		t.Errorf("slice of interface: got %q", v.Items[0].(flat).Email)
	}
}

func TestRegisterAnyTypeMismatch(t *testing.T) {
	tx := transform.New()
	tx.RegisterAny("bad", func(_ any) (any, error) { return 42, nil })
	type obj struct {
		Name string `transform:"bad"`
	}
	v := obj{Name: "hi"}
	err := tx.Transform(&v)
	if err == nil {
		t.Fatal("expected error on RegisterAny type mismatch")
	}
	var fe *transform.FieldError
	if !errors.As(err, &fe) {
		t.Errorf("error should wrap *FieldError, got %T: %v", err, err)
	}
}

func TestTransformArrayField(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items [2]flat
	}
	v := obj{Items: [2]flat{{Email: "a@a.com"}, {Email: "b@b.com"}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Items[0].Email != "A@A.COM" || v.Items[1].Email != "B@B.COM" {
		t.Errorf("array field: got %q %q", v.Items[0].Email, v.Items[1].Email)
	}
}

func TestTransformSliceOfArray(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items [][2]flat
	}
	v := obj{Items: [][2]flat{{{Email: "a@a.com"}, {Email: "b@b.com"}}}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Items[0][0].Email != "A@A.COM" || v.Items[0][1].Email != "B@B.COM" {
		t.Errorf("slice of array: got %q %q", v.Items[0][0].Email, v.Items[0][1].Email)
	}
}

// ========== Tagged compound fields (transform + traverse compose) ==========

func TestTransformEmbeddedStructWithAnyTag(t *testing.T) {
	tx := newTransformer()
	tx.RegisterAny("stamp", func(v any) (any, error) {
		s := v.(StampInner)
		s.Email = "stamped_" + s.Email
		return s, nil
	})
	v := StampOuter{StampInner: StampInner{Email: "alice@example.com", Secret: "12345678"}, ID: "keep"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	// "stamp" runs first (top-down), then the inner "upper"/"mask" tags.
	if v.Email != "STAMPED_ALICE@EXAMPLE.COM" {
		t.Errorf("embedded any: got %q", v.Email)
	}
	if v.Secret != "12****78" {
		t.Errorf("embedded any inner mask: got %q", v.Secret)
	}
	if v.ID != "keep" {
		t.Errorf("ID changed: %q", v.ID)
	}
}

func TestTransformCompoundFieldWithStringTag(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item flat `transform:"upper"` // string fn on a struct field
	}
	v := obj{Item: flat{Email: "alice@example.com", Secret: "12345678"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	// "upper" cannot apply to a struct, but inner tags are still honored.
	if v.Item.Email != "ALICE@EXAMPLE.COM" {
		t.Errorf("compound string tag: got %q", v.Item.Email)
	}
	if v.Item.Secret != "12****78" {
		t.Errorf("compound string tag inner mask: got %q", v.Item.Secret)
	}
}

func TestTransformUnexportedEmbeddedWithAnyTag(t *testing.T) {
	tx := newTransformer()
	tx.RegisterAny("stamp", func(v any) (any, error) {
		// would panic if the fn were invoked on an unsettable field
		t.Fatal("fn should not run on unsettable embedded field")
		return v, nil
	})
	type obj struct {
		flat `transform:"stamp"`
	}
	v := obj{flat: flat{Email: "alice@example.com", Secret: "12345678"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	// The embedded unexported field is read-only, so "stamp" is skipped,
	// but its exported sub-fields are still traversed.
	if v.Email != "ALICE@EXAMPLE.COM" {
		t.Errorf("unexported embedded: got %q", v.Email)
	}
	if v.Secret != "12****78" {
		t.Errorf("unexported embedded mask: got %q", v.Secret)
	}
}

func TestTransformUnexportedEmbeddedInterface(t *testing.T) {
	tx := newTransformer()
	type outer struct {
		hiddenIface
	}
	v := outer{hiddenIface: hiddenImpl{Name: "alice"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	// The embedded unexported interface is read-only, so its content is
	// skipped. Previously this panicked on the write-back.
	if got := v.hiddenIface.(hiddenImpl).Name; got != "alice" {
		t.Errorf("unexported embedded interface content changed: %q", got)
	}
}

func TestTransformUnexportedEmbeddedInterfaceTagged(t *testing.T) {
	tx := newTransformer()
	tx.RegisterAny("stamp", func(v any) (any, error) {
		t.Fatal("fn must not run on unsettable embedded interface")
		return v, nil
	})

	type withStringTag struct {
		hiddenIface `transform:"upper"`
	}
	type withAnyTag struct {
		hiddenIface `transform:"stamp"`
	}

	v1 := withStringTag{hiddenIface: hiddenImpl{Name: "alice"}}
	if err := tx.Transform(&v1); err != nil {
		t.Fatal(err)
	}
	v2 := withStringTag{hiddenIface: hiddenStr("alice")}
	if err := tx.Transform(&v2); err != nil {
		t.Fatal(err)
	}
	v3 := withAnyTag{hiddenIface: hiddenImpl{Name: "bob"}}
	if err := tx.Transform(&v3); err != nil {
		t.Fatal(err)
	}
	if v2.hiddenIface.(hiddenStr) != "alice" {
		t.Errorf("unexported embedded interface content changed: %q", v2.hiddenIface.(hiddenStr))
	}
}

func TestTransformExportedEmbeddedInterface(t *testing.T) {
	tx := newTransformer()
	type outer struct {
		PublicIface
	}
	v := outer{PublicIface: publicIfaceImpl{Name: "alice"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if got := v.PublicIface.(publicIfaceImpl).Name; got != "ALICE" {
		t.Errorf("exported embedded interface not transformed: %q", got)
	}
	if err := tx.Transform(&outer{}); err != nil {
		t.Errorf("nil embedded interface: %v", err)
	}
}

// ========== Validation panics ==========

func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestWithTagEmptyPanics(t *testing.T) {
	mustPanic(t, func() { transform.New(transform.WithTag("")) })
}

func TestRegisterDuplicateKeyPanics(t *testing.T) {
	tx := transform.New()
	tx.RegisterString("upper", strings.ToUpper)
	mustPanic(t, func() { tx.RegisterString("upper", strings.ToLower) })
	mustPanic(t, func() { tx.RegisterStringErr("upper", func(s string) (string, error) { return s, nil }) })
	mustPanic(t, func() { tx.RegisterAny("upper", func(v any) (any, error) { return v, nil }) })
}

func TestRegisterAnyDuplicateKeyPanics(t *testing.T) {
	tx := transform.New()
	tx.RegisterAny("k", func(v any) (any, error) { return v, nil })
	mustPanic(t, func() { tx.RegisterString("k", strings.ToUpper) })
	mustPanic(t, func() { tx.RegisterStringErr("k", func(s string) (string, error) { return s, nil }) })
}

func TestRegisterAnyNilLeavesFieldUnchanged(t *testing.T) {
	tx := transform.New()
	tx.RegisterAny("clear", func(_ any) (any, error) { return nil, nil })
	type obj struct {
		Item interface{}
	}
	v := obj{Item: 42}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Item != 42 {
		t.Errorf("nil result should leave field unchanged, got %v", v.Item)
	}
}

// ========== Pointer-to-compound fields (regression) ==========

func TestTransformPtrToSlice(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items *[]flat
	}
	items := []flat{{Email: "a@a.com"}}
	v := obj{Items: &items}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if items[0].Email != "A@A.COM" {
		t.Errorf("ptr-to-slice: got %q, want A@A.COM", items[0].Email)
	}
}

func TestTransformPtrToMap(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items *map[string]flat
	}
	m := map[string]flat{"a": {Email: "a@a.com"}}
	v := obj{Items: &m}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if m["a"].Email != "A@A.COM" {
		t.Errorf("ptr-to-map: got %q, want A@A.COM", m["a"].Email)
	}
}

func TestTransformPtrToArray(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items *[1]flat
	}
	a := [1]flat{{Email: "a@a.com"}}
	v := obj{Items: &a}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if a[0].Email != "A@A.COM" {
		t.Errorf("ptr-to-array: got %q, want A@A.COM", a[0].Email)
	}
}

func TestTransformNilPtrToSlice(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Items *[]flat
	}
	v := obj{Items: nil}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
}

// ========== String transforms through interfaces (regression) ==========

func TestTransformInterfaceString(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item any `transform:"upper"`
	}
	v := obj{Item: "alice"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Item != "ALICE" {
		t.Errorf("interface string: got %q, want ALICE", v.Item)
	}
}

func TestTransformInterfaceStringErr(t *testing.T) {
	tx := transform.New()
	tx.RegisterStringErr("upper", func(s string) (string, error) {
		return strings.ToUpper(s), nil
	})
	type obj struct {
		Item any `transform:"upper"`
	}
	v := obj{Item: "alice"}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if v.Item != "ALICE" {
		t.Errorf("interface string err: got %q, want ALICE", v.Item)
	}
}

func TestTransformInterfaceStructWithStringKey(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item any `transform:"upper"` // string fn on interface holding a struct
	}
	v := obj{Item: flat{Email: "a@a.com"}}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	// string fn cannot apply to a struct: inner tags still honored
	inner, ok := v.Item.(flat)
	if !ok {
		t.Fatalf("interface type changed: %T", v.Item)
	}
	if inner.Email != "A@A.COM" {
		t.Errorf("interface struct: got %q, want A@A.COM", inner.Email)
	}
}

func TestTransformPtrToInterfaceString(t *testing.T) {
	tx := newTransformer()
	type obj struct {
		Item *any `transform:"upper"`
	}
	item := any("alice")
	v := obj{Item: &item}
	if err := tx.Transform(&v); err != nil {
		t.Fatal(err)
	}
	if *v.Item != "ALICE" {
		t.Errorf("ptr-to-interface string: got %q, want ALICE", *v.Item)
	}
}
