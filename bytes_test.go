package transform_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/goxang/transform"
)

// bytesTransformer registers the byte-slice transforms the tests below share.
func bytesTransformer() *transform.Transformer {
	tr := transform.New()
	tr.RegisterBytes("upperb", bytes.ToUpper)
	tr.RegisterBytes("redact", func(b []byte) []byte { return bytes.Repeat([]byte("*"), len(b)) })
	tr.RegisterBytesErr("upperb_err", func(b []byte) ([]byte, error) { return bytes.ToUpper(b), nil })
	tr.RegisterBytesErr("failb_err", func([]byte) ([]byte, error) { return nil, errors.New("intentional failure") })
	return tr
}

// ========== Direct byte-slice fields ==========

func TestBytesField(t *testing.T) {
	var v struct {
		Payload []byte `transform:"upperb"`
		Other   []byte
	}
	v.Payload = []byte("abc")
	v.Other = []byte("abc")
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("ABC")) {
		t.Errorf("Payload = %q, want ABC", v.Payload)
	}
	if !bytes.Equal(v.Other, []byte("abc")) {
		t.Errorf("untagged Other = %q, want abc", v.Other)
	}
}

func TestBytesFieldLengthMayChange(t *testing.T) {
	tr := transform.New()
	tr.RegisterBytes("trim", func(b []byte) []byte { return bytes.TrimSpace(b) })
	var v struct {
		Payload []byte `transform:"trim"`
	}
	v.Payload = []byte("  hi  ")
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("hi")) {
		t.Errorf("Payload = %q, want hi", v.Payload)
	}
}

func TestBytesFieldNamedType(t *testing.T) {
	type raw []byte
	var v struct {
		Payload raw `transform:"upperb"`
	}
	v.Payload = raw("abc")
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("ABC")) {
		t.Errorf("Payload = %q, want ABC", v.Payload)
	}
}

func TestBytesFieldNamedElem(t *testing.T) {
	type octet byte
	var v struct {
		Payload []octet `transform:"upperb"`
	}
	v.Payload = []octet("abc")
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if string(v.Payload[0]) != "A" {
		t.Errorf("Payload = %v, want ABC", v.Payload)
	}
}

func TestBytesFieldPointer(t *testing.T) {
	b := []byte("abc")
	var v struct {
		Payload *[]byte `transform:"upperb"`
		Nil     *[]byte `transform:"upperb"`
	}
	v.Payload = &b
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(*v.Payload, []byte("ABC")) {
		t.Errorf("Payload = %q, want ABC", *v.Payload)
	}
	if v.Nil != nil {
		t.Error("nil pointer must be left alone")
	}
}

func TestBytesFieldNilAndEmpty(t *testing.T) {
	var v struct {
		Nil   []byte `transform:"upperb"`
		Empty []byte `transform:"upperb"`
	}
	v.Empty = []byte{}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(v.Nil) != 0 || len(v.Empty) != 0 {
		t.Errorf("nil/empty byte slices must stay empty: %v %v", v.Nil, v.Empty)
	}
}

func TestBytesFieldReplacedWithNil(t *testing.T) {
	tr := transform.New()
	tr.RegisterBytes("drop", func([]byte) []byte { return nil })
	var v struct {
		Payload []byte `transform:"drop"`
	}
	v.Payload = []byte("secret")
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Payload != nil {
		t.Errorf("Payload = %v, want nil", v.Payload)
	}
}

func TestBytesFieldNested(t *testing.T) {
	type inner struct {
		Payload []byte `transform:"redact"`
	}
	var v struct {
		In  inner
		Ptr *inner
	}
	v.In.Payload = []byte("abcd")
	v.Ptr = &inner{Payload: []byte("ef")}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.In.Payload, []byte("****")) || !bytes.Equal(v.Ptr.Payload, []byte("**")) {
		t.Errorf("In = %q, Ptr = %q", v.In.Payload, v.Ptr.Payload)
	}
}

type bytesWithUnexported struct {
	Payload []byte `transform:"upperb"`
	secret  []byte //nolint:unused // read back through a method to prove it was not transformed
}

func (b bytesWithUnexported) Secret() []byte { return b.secret }

func TestBytesFieldUnexportedUntouched(t *testing.T) {
	v := bytesWithUnexported{Payload: []byte("a"), secret: []byte("b")}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("A")) {
		t.Errorf("Payload = %q, want A", v.Payload)
	}
	if !bytes.Equal(v.Secret(), []byte("b")) {
		t.Errorf("unexported field = %q, want b", v.Secret())
	}
}

func TestBytesErrField(t *testing.T) {
	var v struct {
		Payload []byte `transform:"upperb_err"`
	}
	v.Payload = []byte("abc")
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("ABC")) {
		t.Errorf("Payload = %q, want ABC", v.Payload)
	}
}

func TestBytesErrPropagates(t *testing.T) {
	var v struct {
		Payload []byte `transform:"failb_err"`
	}
	v.Payload = []byte("abc")
	err := bytesTransformer().Transform(&v)
	if err == nil {
		t.Fatal("expected an error")
	}
	var fe *transform.FieldError
	if !errors.As(err, &fe) || fe.Field != "Payload" || fe.Key != "failb_err" {
		t.Errorf("err = %v, want a FieldError on Payload with key failb_err", err)
	}
	if !bytes.Equal(v.Payload, []byte("abc")) {
		t.Errorf("a failed transform must not write: %q", v.Payload)
	}
}

// ========== Containers of byte slices ==========

func TestBytesContainers(t *testing.T) {
	p := []byte("p")
	var v struct {
		Frames  [][]byte            `transform:"upperb"`
		Named   map[string][]byte   `transform:"upperb"`
		Fixed   [2][]byte           `transform:"upperb"`
		Ptrs    map[string]*[]byte  `transform:"upperb"`
		Deep    map[string][][]byte `transform:"upperb"`
		Ignored [][]byte
	}
	v.Frames = [][]byte{[]byte("a"), []byte("b")}
	v.Named = map[string][]byte{"k": []byte("c")}
	v.Fixed = [2][]byte{[]byte("d"), []byte("e")}
	v.Ptrs = map[string]*[]byte{"k": &p}
	v.Deep = map[string][][]byte{"k": {[]byte("f")}}
	v.Ignored = [][]byte{[]byte("g")}

	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Frames[0], []byte("A")) || !bytes.Equal(v.Frames[1], []byte("B")) {
		t.Errorf("Frames = %q", v.Frames)
	}
	if !bytes.Equal(v.Named["k"], []byte("C")) {
		t.Errorf("Named = %q", v.Named)
	}
	if !bytes.Equal(v.Fixed[0], []byte("D")) || !bytes.Equal(v.Fixed[1], []byte("E")) {
		t.Errorf("Fixed = %q", v.Fixed)
	}
	if !bytes.Equal(*v.Ptrs["k"], []byte("P")) {
		t.Errorf("Ptrs[k] = %q", *v.Ptrs["k"])
	}
	if !bytes.Equal(v.Deep["k"][0], []byte("F")) {
		t.Errorf("Deep[k] = %q", v.Deep["k"])
	}
	if !bytes.Equal(v.Ignored[0], []byte("g")) {
		t.Errorf("untagged container = %q, want g", v.Ignored)
	}
}

func TestBytesContainerKeysNotTransformed(t *testing.T) {
	var v struct {
		Named map[string][]byte `transform:"upperb"`
	}
	v.Named = map[string][]byte{"k": []byte("c")}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if _, ok := v.Named["k"]; !ok {
		t.Errorf("map keys must not be transformed: %v", v.Named)
	}
}

func TestBytesContainerErrPropagates(t *testing.T) {
	var v struct {
		Frames [][]byte `transform:"failb_err"`
	}
	v.Frames = [][]byte{[]byte("a")}
	err := bytesTransformer().Transform(&v)
	var fe *transform.FieldError
	if !errors.As(err, &fe) || fe.Field != "Frames" {
		t.Fatalf("err = %v, want a FieldError on Frames", err)
	}
}

func TestBytesContainerMapErrPropagates(t *testing.T) {
	var v struct {
		Named map[string][]byte `transform:"failb_err"`
	}
	v.Named = map[string][]byte{"k": []byte("a")}
	if err := bytesTransformer().Transform(&v); err == nil {
		t.Fatal("expected an error")
	}
}

func TestBytesContainerNilAndEmpty(t *testing.T) {
	var v struct {
		Nil   [][]byte          `transform:"upperb"`
		Empty map[string][]byte `transform:"upperb"`
	}
	v.Empty = map[string][]byte{}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Nil != nil || len(v.Empty) != 0 {
		t.Error("nil/empty containers must be left alone")
	}
}

func TestBytesInsideStructSlicesAndMaps(t *testing.T) {
	type frame struct {
		Payload []byte `transform:"redact"`
	}
	var v struct {
		List []frame
		ByID map[string]frame
	}
	v.List = []frame{{Payload: []byte("ab")}}
	v.ByID = map[string]frame{"k": {Payload: []byte("cde")}}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.List[0].Payload, []byte("**")) {
		t.Errorf("List[0] = %q, want **", v.List[0].Payload)
	}
	if !bytes.Equal(v.ByID["k"].Payload, []byte("***")) {
		t.Errorf("ByID[k] = %q, want ***", v.ByID["k"].Payload)
	}
}

// ========== Types a bytes transform does not apply to ==========

func TestBytesTransformLeavesByteArrayAlone(t *testing.T) {
	var v struct {
		Sum [4]byte `transform:"upperb"`
	}
	v.Sum = [4]byte{'a', 'b', 'c', 'd'}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Sum != [4]byte{'a', 'b', 'c', 'd'} {
		t.Errorf("Sum = %q, want abcd: a byte array is not a byte slice", v.Sum)
	}
}

func TestBytesTransformLeavesOtherTypesAlone(t *testing.T) {
	var v struct {
		Name string   `transform:"upperb"`
		N    int      `transform:"upperb"`
		Nums []int    `transform:"upperb"`
		Strs []string `transform:"upperb"`
	}
	v.Name, v.N, v.Nums, v.Strs = "a", 1, []int{2}, []string{"b"}
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if v.Name != "a" || v.N != 1 || v.Nums[0] != 2 || v.Strs[0] != "b" {
		t.Errorf("unexpected result: %+v", v)
	}
}

func TestStringTransformLeavesBytesAlone(t *testing.T) {
	tr := newTransformer()
	var v struct {
		Payload []byte `transform:"upper"`
	}
	v.Payload = []byte("abc")
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("abc")) {
		t.Errorf("Payload = %q, want abc: a string transform does not apply to []byte", v.Payload)
	}
}

func TestRegisterAnyStillReachesBytes(t *testing.T) {
	tr := transform.New()
	tr.RegisterAny("wrap", func(v any) (any, error) {
		return append([]byte("x_"), v.([]byte)...), nil
	})
	var v struct {
		Payload []byte `transform:"wrap"`
	}
	v.Payload = []byte("a")
	if err := tr.Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("x_a")) {
		t.Errorf("Payload = %q, want x_a", v.Payload)
	}
}

// ========== Interface fields ==========

func TestBytesTransformOnInterfaceField(t *testing.T) {
	var v struct {
		Held  any `transform:"upperb"`
		Other any `transform:"upperb"`
		Nil   any `transform:"upperb"`
	}
	v.Held = []byte("abc")
	v.Other = 42
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	held, ok := v.Held.([]byte)
	if !ok || !bytes.Equal(held, []byte("ABC")) {
		t.Errorf("Held = %v, want ABC", v.Held)
	}
	if v.Other != 42 || v.Nil != nil {
		t.Errorf("non-bytes interface values must be left alone: %v %v", v.Other, v.Nil)
	}
}

func TestBytesErrTransformOnInterfaceField(t *testing.T) {
	var v struct {
		Held any `transform:"upperb_err"`
		Bad  any `transform:"failb_err"`
	}
	v.Held = []byte("abc")
	if err := bytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	held, _ := v.Held.([]byte)
	if !bytes.Equal(held, []byte("ABC")) {
		t.Errorf("Held = %v, want ABC", v.Held)
	}

	v.Bad = []byte("x")
	if err := bytesTransformer().Transform(&v); err == nil {
		t.Fatal("expected the failing transform to propagate")
	}
}

// ========== Strict mode ==========

func strictBytesTransformer() *transform.Transformer {
	tr := transform.New(transform.WithStrict())
	tr.RegisterBytes("upperb", bytes.ToUpper)
	return tr
}

func TestStrictBytesUnusableKey(t *testing.T) {
	cases := map[string]any{
		"string": &struct {
			S string `transform:"upperb"`
		}{},
		"scalar": &struct {
			N int `transform:"upperb"`
		}{},
		"byteArray": &struct {
			A [4]byte `transform:"upperb"`
		}{},
		"stringSlice": &struct {
			S []string `transform:"upperb"`
		}{},
		"struct": &struct {
			S struct{ A string } `transform:"upperb"`
		}{},
		"unexported": &struct {
			payload []byte `transform:"upperb"`
		}{},
	}
	for name, v := range cases {
		if err := strictBytesTransformer().Transform(v); !errors.Is(err, transform.ErrUnusableKey) {
			t.Errorf("%s: err = %v, want ErrUnusableKey", name, err)
		}
	}
}

func TestStrictBytesAcceptsValidTags(t *testing.T) {
	var v struct {
		Payload []byte            `transform:"upperb"`
		Frames  [][]byte          `transform:"upperb"`
		Named   map[string][]byte `transform:"upperb"`
		Held    any               `transform:"upperb"`
	}
	v.Payload, v.Frames = []byte("a"), [][]byte{[]byte("b")}
	v.Named, v.Held = map[string][]byte{"k": []byte("c")}, []byte("d")
	if err := strictBytesTransformer().Transform(&v); err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if !bytes.Equal(v.Payload, []byte("A")) || !bytes.Equal(v.Frames[0], []byte("B")) {
		t.Errorf("unexpected result: %+v", v)
	}
}

// ========== Registration ==========

func TestRegisterBytesEmptyKeyPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic")
		}
	}()
	transform.New().RegisterBytes("", bytes.ToUpper)
}

func TestRegisterBytesNilFuncPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic")
		}
	}()
	transform.New().RegisterBytes("k", nil)
}

func TestRegisterBytesErrEmptyKeyPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic")
		}
	}()
	transform.New().RegisterBytesErr("", func(b []byte) ([]byte, error) { return b, nil })
}

func TestRegisterBytesErrNilFuncPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic")
		}
	}()
	transform.New().RegisterBytesErr("k", nil)
}

func TestRegisterBytesDuplicateKeyPanics(t *testing.T) {
	cases := map[string]func(*transform.Transformer){
		"bytes":  func(tr *transform.Transformer) { tr.RegisterBytes("dup", bytes.ToUpper) },
		"string": func(tr *transform.Transformer) { tr.RegisterString("dup", func(s string) string { return s }) },
		"any":    func(tr *transform.Transformer) { tr.RegisterAny("dup", func(v any) (any, error) { return v, nil }) },
	}
	for name, second := range cases {
		t.Run(name, func(t *testing.T) {
			tr := transform.New()
			tr.RegisterBytes("dup", bytes.ToUpper)
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected a panic on the duplicate key")
				}
			}()
			second(tr)
		})
	}
}

func TestRegisterOnZeroValuePanics(t *testing.T) {
	cases := map[string]func(*transform.Transformer){
		"string": func(tr *transform.Transformer) { tr.RegisterString("k", func(s string) string { return s }) },
		"stringErr": func(tr *transform.Transformer) {
			tr.RegisterStringErr("k", func(s string) (string, error) { return s, nil })
		},
		"bytes": func(tr *transform.Transformer) { tr.RegisterBytes("k", bytes.ToUpper) },
		"bytesErr": func(tr *transform.Transformer) {
			tr.RegisterBytesErr("k", func(b []byte) ([]byte, error) { return b, nil })
		},
		"any": func(tr *transform.Transformer) { tr.RegisterAny("k", func(v any) (any, error) { return v, nil }) },
	}
	for name, register := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected a panic: the zero value is not a usable Transformer")
				}
			}()
			register(new(transform.Transformer))
		})
	}
}

func TestRegisterBytesAfterTransformPanics(t *testing.T) {
	tr := transform.New()
	tr.RegisterBytes("upperb", bytes.ToUpper)
	var v struct {
		Payload []byte `transform:"upperb"`
	}
	_ = tr.Transform(&v)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic")
		}
	}()
	tr.RegisterBytes("another", bytes.ToUpper)
}

func TestRegisterBytesErrAfterTransformPanics(t *testing.T) {
	tr := transform.New()
	tr.RegisterBytes("upperb", bytes.ToUpper)
	var v struct {
		Payload []byte `transform:"upperb"`
	}
	_ = tr.Transform(&v)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected a panic")
		}
	}()
	tr.RegisterBytesErr("another", func(b []byte) ([]byte, error) { return b, nil })
}
