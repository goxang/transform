// Package transform applies registered functions to struct fields selected by
// struct tags, in place, in a single call.
//
// The point is to keep normalization rules (trim, lowercase, redact, ...) next
// to the fields they apply to, instead of in a hand-written recursive walk you
// have to remember to update every time a struct grows a field.
//
// # Basic usage
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
//
// # Traversal
//
// Transform recurses into nested structs, pointers, slices, arrays, maps, and
// interfaces, so tagged fields anywhere in the value graph are reached. Only
// exported fields are transformed, and map keys never are — only values.
//
// A string transform tagged on a container of strings ([]string,
// map[K]string, [N]string, and any nesting of those) is applied to every
// element.
//
// # Lifecycle
//
// A Transformer is configured once and then read-only. The first Transform
// call freezes it; any later Register call panics. Once frozen it is safe to
// share across goroutines: the per-type plan cache is a sync.Map and the
// transform path only reads the registries.
//
// The zero value is not usable — use [New].
//
// # Performance
//
// The first Transform of a given type builds an execution plan (reads fields
// and tags, resolves the registered functions, compiles one closure per field,
// drops the fields with nothing to do) and caches it by reflect.Type. Every
// later call is a loop over those closures, with no tag parsing and no
// registry lookups.
//
// # Failure modes
//
// Transform stops at the first error and wraps it in a [*FieldError] naming
// the struct type, field, and key. There is no rollback.
//
// Two options guard the common ways a tag-driven API goes wrong:
// [WithStrict] turns an unresolvable tag (a typo, a json-style ",omitempty"
// suffix, a key that cannot apply to the field's type) into an error instead
// of a silent no-op, and [WithMaxDepth] bounds traversal so cyclic data is
// reported as [ErrMaxDepth] rather than overflowing the stack.
package transform
