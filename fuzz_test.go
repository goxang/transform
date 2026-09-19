package transform_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/goxang/transform"
)

func FuzzTransform(f *testing.F) {
	f.Add("hello", 42)
	f.Add("world", 100)
	f.Add("", 0)

	f.Fuzz(func(t *testing.T, name string, age int) {
		tx := transform.New()
		tx.RegisterString("upper", func(s string) string {
			result := []byte(s)
			for i := range result {
				if result[i] >= 'a' && result[i] <= 'z' {
					result[i] -= 32
				}
			}
			return string(result)
		})

		type obj struct {
			Name string `transform:"upper"`
			Age  int
		}
		v := obj{Name: name, Age: age}
		if err := tx.Transform(&v); err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
	})
}

func FuzzTransformNested(f *testing.F) {
	f.Add("alice", "bob@example.com", "secret!", 2)
	f.Add("", "", "", 0)
	f.Add("Bob", "CAROL", "12345678", -1)
	f.Add("a", "b", "c", 1)

	f.Fuzz(func(t *testing.T, a, b, c string, n int) {
		tx := transform.New()
		tx.RegisterString("upper", strings.ToUpper)
		tx.RegisterString("mask", func(s string) string {
			if len(s) <= 4 {
				return "****"
			}
			return s[:2] + "****" + s[len(s)-2:]
		})
		tx.RegisterAny("double", func(v any) (any, error) {
			return v.(int) * 2, nil
		})

		type inner struct {
			Name   string `transform:"upper"`
			Secret string `transform:"mask"`
			Count  int    `transform:"double"`
		}
		type outer struct {
			List []inner
			Dict map[string]*inner
			Item any
			Ptr  *inner
			Arr  [2]inner
		}
		v := outer{
			List: []inner{{Name: a, Secret: c, Count: n}, {Name: b, Secret: c, Count: n + 1}},
			Dict: map[string]*inner{"x": {Name: c, Secret: a, Count: n}},
			Item: inner{Name: a, Secret: b, Count: n},
			Ptr:  &inner{Name: b, Secret: c, Count: n},
			Arr:  [2]inner{{Name: c, Secret: a, Count: n}, {Name: a, Secret: b, Count: n}},
		}
		if err := tx.Transform(&v); err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
		if v.List[0].Name != strings.ToUpper(a) || v.List[0].Count != n*2 {
			t.Errorf("List[0]: got %q %d", v.List[0].Name, v.List[0].Count)
		}
		if v.List[1].Name != strings.ToUpper(b) || v.List[1].Count != (n+1)*2 {
			t.Errorf("List[1]: got %q %d", v.List[1].Name, v.List[1].Count)
		}
		if v.Dict["x"].Name != strings.ToUpper(c) {
			t.Errorf("Dict: got %q", v.Dict["x"].Name)
		}
		item, ok := v.Item.(inner)
		if !ok {
			t.Fatalf("Item type changed: %T", v.Item)
		}
		if item.Name != strings.ToUpper(a) || item.Count != n*2 {
			t.Errorf("Item: got %q %d", item.Name, item.Count)
		}
		if v.Ptr.Name != strings.ToUpper(b) || v.Ptr.Count != n*2 {
			t.Errorf("Ptr: got %q %d", v.Ptr.Name, v.Ptr.Count)
		}
		if v.Arr[0].Name != strings.ToUpper(c) || v.Arr[1].Count != n*2 {
			t.Errorf("Arr: got %q %d %q %d", v.Arr[0].Name, v.Arr[0].Count, v.Arr[1].Name, v.Arr[1].Count)
		}
	})
}

// FuzzTransformBytesContainers is the byte-slice counterpart: a bytes
// transform tagged on a byte slice and on containers of them must reach every
// slice, including the ones a transform returns at a different length, and
// leave keys and shapes untouched.
func FuzzTransformBytesContainers(f *testing.F) {
	f.Add([]byte("alpha"), []byte("beta"), "key")
	f.Add([]byte{}, []byte(nil), "")
	f.Add([]byte{0x00, 0xff}, []byte("ß"), "\x00")

	f.Fuzz(func(t *testing.T, a, b []byte, k string) {
		tx := transform.New()
		tx.RegisterBytes("upperb", bytes.ToUpper)
		tx.RegisterBytes("trim", bytes.TrimSpace)

		type obj struct {
			Payload []byte              `transform:"upperb"`
			Trimmed []byte              `transform:"trim"`
			List    [][]byte            `transform:"upperb"`
			Arr     [2][]byte           `transform:"upperb"`
			Dict    map[string][]byte   `transform:"upperb"`
			Multi   map[string][][]byte `transform:"upperb"`
			Held    any                 `transform:"upperb"`
		}
		v := obj{
			Payload: a,
			Trimmed: b,
			List:    [][]byte{a, b},
			Arr:     [2][]byte{a, b},
			Dict:    map[string][]byte{k: a},
			Multi:   map[string][][]byte{k: {b}},
			Held:    a,
		}
		if err := tx.Transform(&v); err != nil {
			t.Fatalf("Transform failed: %v", err)
		}

		wantA, wantB := bytes.ToUpper(a), bytes.ToUpper(b)
		if !bytes.Equal(v.Payload, wantA) {
			t.Errorf("Payload: got %q, want %q", v.Payload, wantA)
		}
		if !bytes.Equal(v.Trimmed, bytes.TrimSpace(b)) {
			t.Errorf("Trimmed: got %q", v.Trimmed)
		}
		if !bytes.Equal(v.List[0], wantA) || !bytes.Equal(v.List[1], wantB) {
			t.Errorf("List: got %q %q", v.List[0], v.List[1])
		}
		if !bytes.Equal(v.Arr[0], wantA) || !bytes.Equal(v.Arr[1], wantB) {
			t.Errorf("Arr: got %q %q", v.Arr[0], v.Arr[1])
		}
		if got, ok := v.Dict[k]; !ok || !bytes.Equal(got, wantA) {
			t.Errorf("Dict[%q]: got %q ok=%v — keys must be preserved verbatim", k, got, ok)
		}
		if !bytes.Equal(v.Multi[k][0], wantB) {
			t.Errorf("Multi: got %q", v.Multi[k])
		}
		held, ok := v.Held.([]byte)
		if !ok || !bytes.Equal(held, wantA) {
			t.Errorf("Held: got %v, want %q", v.Held, wantA)
		}
	})
}

// FuzzTransformStringContainers exercises the container path: a string
// transform tagged on slices, arrays, and maps of strings must reach every
// element and leave the container's shape and keys untouched.
func FuzzTransformStringContainers(f *testing.F) {
	f.Add("alpha", "beta", "key")
	f.Add("", "", "")
	f.Add("ß", "İ", "\x00")

	f.Fuzz(func(t *testing.T, a, b, k string) {
		tx := transform.New()
		tx.RegisterString("upper", strings.ToUpper)

		type obj struct {
			List  []string            `transform:"upper"`
			Arr   [2]string           `transform:"upper"`
			Dict  map[string]string   `transform:"upper"`
			Deep  [][]string          `transform:"upper"`
			Multi map[string][]string `transform:"upper"`
		}
		v := obj{
			List:  []string{a, b},
			Arr:   [2]string{a, b},
			Dict:  map[string]string{k: a},
			Deep:  [][]string{{a}, {b}},
			Multi: map[string][]string{k: {b}},
		}
		if err := tx.Transform(&v); err != nil {
			t.Fatalf("Transform failed: %v", err)
		}

		wantA, wantB := strings.ToUpper(a), strings.ToUpper(b)
		if v.List[0] != wantA || v.List[1] != wantB {
			t.Errorf("List: got %q %q", v.List[0], v.List[1])
		}
		if v.Arr[0] != wantA || v.Arr[1] != wantB {
			t.Errorf("Arr: got %q %q", v.Arr[0], v.Arr[1])
		}
		if got, ok := v.Dict[k]; !ok || got != wantA {
			t.Errorf("Dict[%q]: got %q ok=%v — keys must be preserved verbatim", k, got, ok)
		}
		if v.Deep[0][0] != wantA || v.Deep[1][0] != wantB {
			t.Errorf("Deep: got %v", v.Deep)
		}
		if v.Multi[k][0] != wantB {
			t.Errorf("Multi: got %v", v.Multi)
		}
	})
}
