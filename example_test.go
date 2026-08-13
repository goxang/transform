package transform_test

import (
	"fmt"
	"strings"

	"github.com/MohammadmahdiAhmadi/transform"
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
