package transform_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/goxang/transform"
)

// ========== String transforms on containers of strings ==========

func TestStringSliceField(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Tags []string `transform:"upper"`
	}
	v.Tags = []string{"a", "b", ""}
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	want := []string{"A", "B", ""}
	for i := range want {
		if v.Tags[i] != want[i] {
			t.Errorf("Tags[%d] = %q, want %q", i, v.Tags[i], want[i])
		}
	}
}

func TestStringArrayField(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Codes [2]string `transform:"upper"`
	}
	v.Codes = [2]string{"x", "y"}
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Codes != [2]string{"X", "Y"} {
		t.Errorf("Codes = %v, want [X Y]", v.Codes)
	}
}

func TestStringMapValues(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Meta map[string]string `transform:"upper"`
	}
	v.Meta = map[string]string{"k": "v", "other": "w"}
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Meta["k"] != "V" || v.Meta["other"] != "W" {
		t.Errorf("Meta = %v, want map[k:V other:W]", v.Meta)
	}
	if _, ok := v.Meta["k"]; !ok {
		t.Error("map keys must not be transformed")
	}
}

func TestStringContainerNested(t *testing.T) {
	tr := newTransformer()
	s := "p"
	var v struct {
		Grid [][]string          `transform:"upper"`
		Ptrs map[string]*string  `transform:"upper"`
		Deep map[string][]string `transform:"upper"`
	}
	v.Grid = [][]string{{"a"}, {"b", "c"}}
	v.Ptrs = map[string]*string{"k": &s}
	v.Deep = map[string][]string{"k": {"d"}}

	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Grid[0][0] != "A" || v.Grid[1][1] != "C" {
		t.Errorf("Grid = %v", v.Grid)
	}
	if *v.Ptrs["k"] != "P" {
		t.Errorf("Ptrs[k] = %q, want P", *v.Ptrs["k"])
	}
	if v.Deep["k"][0] != "D" {
		t.Errorf("Deep[k] = %v", v.Deep["k"])
	}
}

func TestStringContainerNamedElem(t *testing.T) {
	type tag string
	tr := newTransformer()
	var v struct {
		Tags []tag `transform:"upper"`
	}
	v.Tags = []tag{"a"}
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Tags[0] != "A" {
		t.Errorf("Tags[0] = %q, want A", v.Tags[0])
	}
}

func TestStringContainerNilAndEmpty(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Nil   []string          `transform:"upper"`
		Empty map[string]string `transform:"upper"`
	}
	v.Empty = map[string]string{}
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Nil != nil || len(v.Empty) != 0 {
		t.Errorf("nil/empty containers must be left alone")
	}
}

func TestStringContainerErrPropagates(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Tags []string `transform:"fail_err"`
	}
	v.Tags = []string{"a"}
	err := tr.Transform(&v)
	if err == nil {
		t.Fatal("expected an error")
	}
	var fe *transform.FieldError
	if !errors.As(err, &fe) || fe.Field != "Tags" {
		t.Errorf("err = %v, want a FieldError on Tags", err)
	}
}

func TestStringContainerNonStringElemUntouched(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Nums []int `transform:"upper"`
	}
	v.Nums = []int{1, 2}
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Nums[0] != 1 || v.Nums[1] != 2 {
		t.Errorf("Nums = %v, want [1 2]", v.Nums)
	}
}

// ========== Depth limit ==========

type cycleNode struct {
	Name string `transform:"upper"`
	Next *cycleNode
}

func TestCyclicPointerDataIsBounded(t *testing.T) {
	tr := newTransformer()
	n := &cycleNode{Name: "a"}
	n.Next = n
	err := tr.Transform(n)
	if !errors.Is(err, transform.ErrMaxDepth) {
		t.Fatalf("err = %v, want ErrMaxDepth", err)
	}
	// The error must not carry one wrapper per frame.
	if len(err.Error()) > 200 {
		t.Errorf("ErrMaxDepth message is %d bytes; it should propagate unwrapped", len(err.Error()))
	}
}

func TestCyclicInterfaceDataIsBounded(t *testing.T) {
	tr := newTransformer()
	l := make([]any, 1)
	l[0] = &l
	if err := tr.Transform(&l); !errors.Is(err, transform.ErrMaxDepth) {
		t.Fatalf("err = %v, want ErrMaxDepth", err)
	}
}

func TestWithMaxDepth(t *testing.T) {
	type l3 struct {
		V string `transform:"upper"`
	}
	type l2 struct{ C l3 }
	type l1 struct{ B l2 }
	type l0 struct{ A l1 }

	deep := transform.New(transform.WithMaxDepth(2))
	deep.RegisterString("upper", strings.ToUpper)
	v := l0{A: l1{B: l2{C: l3{V: "x"}}}}
	if err := deep.Transform(&v); !errors.Is(err, transform.ErrMaxDepth) {
		t.Fatalf("err = %v, want ErrMaxDepth", err)
	}

	roomy := transform.New(transform.WithMaxDepth(10))
	roomy.RegisterString("upper", strings.ToUpper)
	v = l0{A: l1{B: l2{C: l3{V: "x"}}}}
	if err := roomy.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.A.B.C.V != "X" {
		t.Errorf("V = %q, want X", v.A.B.C.V)
	}
}

func TestWithMaxDepthNonPositivePanics(t *testing.T) {
	for _, d := range []int{0, -1} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("WithMaxDepth(%d) did not panic", d)
				}
			}()
			transform.WithMaxDepth(d)
		}()
	}
}

// ========== Strict mode ==========

func strictTransformer() *transform.Transformer {
	tr := transform.New(transform.WithStrict())
	tr.RegisterString("upper", strings.ToUpper)
	tr.RegisterAny("any", func(v any) (any, error) { return v, nil })
	return tr
}

func TestStrictUnknownKey(t *testing.T) {
	var v struct {
		Name string `transform:"typo"`
	}
	err := strictTransformer().Transform(&v)
	if !errors.Is(err, transform.ErrUnknownKey) {
		t.Fatalf("err = %v, want ErrUnknownKey", err)
	}
	var fe *transform.FieldError
	if !errors.As(err, &fe) || fe.Field != "Name" || fe.Key != "typo" {
		t.Errorf("err = %v, want a FieldError on Name with key typo", err)
	}
}

func TestStrictJSONStyleTagOptions(t *testing.T) {
	var v struct {
		Name string `transform:"upper,omitempty"`
	}
	if err := strictTransformer().Transform(&v); !errors.Is(err, transform.ErrUnknownKey) {
		t.Fatalf("err = %v, want ErrUnknownKey", err)
	}
}

func TestStrictUnusableKey(t *testing.T) {
	cases := map[string]any{
		"scalar": &struct {
			N int `transform:"upper"`
		}{},
		"struct": &struct {
			S struct{ A string } `transform:"upper"`
		}{},
		"intSlice": &struct {
			S []int `transform:"upper"`
		}{},
		"intMap": &struct {
			M map[string]int `transform:"upper"`
		}{},
		"unexported": &struct {
			name string `transform:"upper"`
		}{},
	}
	for name, v := range cases {
		if err := strictTransformer().Transform(v); !errors.Is(err, transform.ErrUnusableKey) {
			t.Errorf("%s: err = %v, want ErrUnusableKey", name, err)
		}
	}
}

func TestStrictAcceptsValidTags(t *testing.T) {
	type inner struct {
		B string `transform:"upper"`
	}
	var v struct {
		A     string   `transform:"upper"`
		L     []string `transform:"upper"`
		N     inner
		Any   int `transform:"any"`
		Plain int
		Iface any `transform:"upper"`
	}
	v.A, v.L, v.N.B, v.Iface = "a", []string{"b"}, "c", "d"
	if err := strictTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.A != "A" || v.L[0] != "B" || v.N.B != "C" || v.Iface != "D" {
		t.Errorf("unexpected result: %+v", v)
	}
}

func TestNonStrictIgnoresBadTags(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Name string `transform:"typo"`
		N    int    `transform:"upper"`
	}
	v.Name = "a"
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Name != "a" {
		t.Errorf("Name = %q, want a", v.Name)
	}
}

func TestStrictRepeatedCallsStayConsistent(t *testing.T) {
	tr := strictTransformer()
	var v struct {
		Name string `transform:"typo"`
	}
	for i := 0; i < 3; i++ {
		if !errors.Is(tr.Transform(&v), transform.ErrUnknownKey) {
			t.Fatalf("call %d did not report ErrUnknownKey", i)
		}
	}
}

// ========== TransformValue ==========

type tvUser struct {
	Name string `transform:"upper"`
	Age  int
}

func TestTransformValuePointer(t *testing.T) {
	v := tvUser{Name: "alice"}
	if err := newTransformer().TransformValue(reflect.ValueOf(&v)); err != nil {
		t.Fatalf("TransformValue: %v", err)
	}
	if v.Name != "ALICE" {
		t.Errorf("Name = %q, want ALICE", v.Name)
	}
}

func TestTransformValueAddressableElem(t *testing.T) {
	v := tvUser{Name: "alice"}
	if err := newTransformer().TransformValue(reflect.ValueOf(&v).Elem()); err != nil {
		t.Fatalf("TransformValue: %v", err)
	}
	if v.Name != "ALICE" {
		t.Errorf("Name = %q, want ALICE", v.Name)
	}
}

func TestTransformValueInterfaceHoldingPointer(t *testing.T) {
	v := tvUser{Name: "alice"}
	var boxed any = &v
	if err := newTransformer().TransformValue(reflect.ValueOf(&boxed).Elem()); err != nil {
		t.Fatalf("TransformValue: %v", err)
	}
	if v.Name != "ALICE" {
		t.Errorf("Name = %q, want ALICE", v.Name)
	}
}

func TestTransformValueUnaddressableStructIsLeftAlone(t *testing.T) {
	v := tvUser{Name: "alice"}
	if err := newTransformer().TransformValue(reflect.ValueOf(v)); err != nil {
		t.Fatalf("TransformValue: %v", err)
	}
	if v.Name != "alice" {
		t.Errorf("Name = %q, want alice: a struct passed by value must not be written", v.Name)
	}
}

func TestTransformValueUnaddressableSliceAndMap(t *testing.T) {
	tr := newTransformer()

	items := []tvUser{{Name: "alice"}, {Name: "bob"}}
	if err := tr.TransformValue(reflect.ValueOf(items)); err != nil {
		t.Fatalf("TransformValue slice: %v", err)
	}
	if items[0].Name != "ALICE" || items[1].Name != "BOB" {
		t.Errorf("items = %v, want ALICE and BOB", items)
	}

	m := map[string]tvUser{"a": {Name: "alice"}}
	if err := tr.TransformValue(reflect.ValueOf(m)); err != nil {
		t.Fatalf("TransformValue map: %v", err)
	}
	if m["a"].Name != "ALICE" {
		t.Errorf("m[a] = %v, want ALICE", m["a"])
	}
}

func TestTransformValueUnaddressableArray(t *testing.T) {
	v := [2]tvUser{{Name: "alice"}, {Name: "bob"}}
	if err := newTransformer().TransformValue(reflect.ValueOf(v)); err != nil {
		t.Fatalf("TransformValue: %v", err)
	}
	if v[0].Name != "alice" {
		t.Errorf("v[0].Name = %q, want alice: an array passed by value must not be written", v[0].Name)
	}
}

func TestTransformValueNothingToDo(t *testing.T) {
	tr := newTransformer()
	var nilPtr *tvUser
	var nilMap map[string]tvUser
	cases := map[string]reflect.Value{
		"zero Value":  {},
		"nil pointer": reflect.ValueOf(nilPtr),
		"nil map":     reflect.ValueOf(nilMap),
		"int":         reflect.ValueOf(42),
		"string":      reflect.ValueOf("alice"),
	}
	for name, val := range cases {
		if err := tr.TransformValue(val); err != nil {
			t.Errorf("%s: TransformValue = %v, want nil", name, err)
		}
	}
}

func TestTransformValueCyclicDataIsBounded(t *testing.T) {
	type node struct {
		Name string `transform:"upper"`
		Next *node
	}
	n := &node{Name: "loop"}
	n.Next = n
	if err := newTransformer().TransformValue(reflect.ValueOf(n)); !errors.Is(err, transform.ErrMaxDepth) {
		t.Errorf("TransformValue = %v, want ErrMaxDepth", err)
	}
}
