package transform_test

import (
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
