// Package transform provides reflection-based struct field transformation
// with cached metadata and a dynamic function registry.
//
// Fields are matched to transformation functions via struct tags. Reflection
// metadata is resolved once per type and cached, so repeated transformations
// are fast and allocation-light.
//
// Basic usage:
//
//	t := transform.New()
//	t.RegisterString("upper", strings.ToUpper)
//
//	type User struct {
//	    Name string `transform:"upper"`
//	}
//	u := User{Name: "alice"}
//	if err := t.Transform(&u); err != nil {
//	    log.Fatal(err)
//	}
//	// u.Name == "ALICE"
package transform
