package transform_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/goxang/transform"
)

func Example() {
	t := transform.New()
	t.RegisterString("upper", strings.ToUpper)
	t.RegisterString("lower", strings.ToLower)

	type User struct {
		Name  string `transform:"upper"`
		Email string `transform:"lower"`
		Age   int
	}
	u := User{Name: "Alice", Email: "ALICE@EXAMPLE.COM", Age: 30}
	if err := t.Transform(&u); err != nil {
		panic(err)
	}
	fmt.Println(u.Name, u.Email, u.Age)
	// Output: ALICE alice@example.com 30
}

func ExampleTransformer_RegisterStringErr() {
	t := transform.New()
	t.RegisterStringErr("parse", func(s string) (string, error) {
		return strings.ToUpper(s), nil
	})

	type User struct {
		Name string `transform:"parse"`
	}
	u := User{Name: "alice"}
	if err := t.Transform(&u); err != nil {
		panic(err)
	}
	fmt.Println(u.Name)
	// Output: ALICE
}

func ExampleTransformer_RegisterBytes() {
	t := transform.New()
	t.RegisterBytes("redact", func(b []byte) []byte {
		return bytes.Repeat([]byte("*"), len(b))
	})

	type Record struct {
		Key    []byte   `transform:"redact"`
		Frames [][]byte `transform:"redact"`
	}
	r := Record{Key: []byte("hunter2"), Frames: [][]byte{[]byte("ab"), []byte("cde")}}
	if err := t.Transform(&r); err != nil {
		panic(err)
	}
	fmt.Printf("%s %s\n", r.Key, r.Frames)
	// Output: ******* [** ***]
}

func ExampleTransformer_RegisterBytesErr() {
	t := transform.New()
	t.RegisterBytesErr("decode", func(b []byte) ([]byte, error) {
		out := make([]byte, hex.DecodedLen(len(b)))
		n, err := hex.Decode(out, b)
		return out[:n], err
	})

	type Frame struct {
		Body []byte `transform:"decode"`
	}
	f := Frame{Body: []byte("6f6b")}
	if err := t.Transform(&f); err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", f.Body)
	// Output: ok
}

func ExampleTransformer_RegisterAny() {
	t := transform.New()
	t.RegisterAny("double", func(v any) (any, error) {
		n, ok := v.(int)
		if !ok {
			return nil, fmt.Errorf("expected int, got %T", v)
		}
		return n * 2, nil
	})

	type Counter struct {
		Value int `transform:"double"`
	}
	c := Counter{Value: 21}
	if err := t.Transform(&c); err != nil {
		panic(err)
	}
	fmt.Println(c.Value)
	// Output: 42
}

func ExampleTransformer_Transform_stringSlice() {
	t := transform.New()
	t.RegisterString("trim", strings.TrimSpace)

	type Post struct {
		Tags   []string          `transform:"trim"`
		Fields map[string]string `transform:"trim"`
	}
	p := Post{
		Tags:   []string{" go ", " reflect "},
		Fields: map[string]string{"title": "  Hello  "},
	}
	if err := t.Transform(&p); err != nil {
		panic(err)
	}
	fmt.Printf("%q %q\n", p.Tags, p.Fields["title"])
	// Output: ["go" "reflect"] "Hello"
}

func ExampleWithStrict() {
	t := transform.New(transform.WithStrict())
	t.RegisterString("upper", strings.ToUpper)

	type User struct {
		Name string `transform:"uppr"` // typo
	}
	err := t.Transform(&User{})
	fmt.Println(err)
	fmt.Println(errors.Is(err, transform.ErrUnknownKey))
	// Output:
	// transform User.Name (key "uppr"): no transformation registered for key
	// true
}

func ExampleWithMaxDepth() {
	type Node struct {
		Name string `transform:"upper"`
		Next *Node
	}
	t := transform.New()
	t.RegisterString("upper", strings.ToUpper)

	n := &Node{Name: "loop"}
	n.Next = n // cyclic data

	err := t.Transform(n)
	fmt.Println(errors.Is(err, transform.ErrMaxDepth))
	// Output: true
}
