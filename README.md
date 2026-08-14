# transform

[![Go Reference](https://pkg.go.dev/badge/github.com/goxang/transform.svg)](https://pkg.go.dev/github.com/goxang/transform)
[![CI](https://github.com/goxang/transform/actions/workflows/ci.yml/badge.svg)](https://github.com/goxang/transform/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-84.8%25-brightgreen)](https://github.com/goxang/transform/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**Declarative data transformation for Go structs.**

Describe transformations next to your fields with struct tags, then apply
them to the whole value — including nested structs, pointers, slices, arrays,
maps, and interfaces — with a single call. Transformations are user-defined:
you register functions by name, and a struct tag selects which one runs.

```go
t := transform.New()
t.RegisterString("normalize", func(s string) string {
    return strings.ToLower(strings.TrimSpace(s))
})
t.RegisterString("trim", strings.TrimSpace)

type User struct {
    Email string `transform:"normalize"`
    Name  string `transform:"trim"`
}

u := User{Email: "  ALICE@Example.COM ", Name: " Alice "}
_ = t.Transform(&u)
// u.Email == "alice@example.com"
// u.Name  == "Alice"
```

One call walks the whole value — no manual recursive field walking.

**At a glance**

- Declarative struct-tag based transformations
- Recursive traversal of nested values
- In-place mutation (no copies)
- Cached reflection metadata
- User-defined transformation registry
- Concurrent use after setup
- Zero dependencies
- No code generation

## Why transform?

The rule should live next to the field, not in a walking function you have to
write and maintain.

**Manual**

```go
user.Name = strings.TrimSpace(user.Name)
user.Email = normalizeEmail(user.Email)

user.Address.City = strings.TrimSpace(user.Address.City)

for i := range user.Contacts {
    user.Contacts[i].Email = normalizeEmail(user.Contacts[i].Email)
}
```

Every new field means another line to write, and another place to remember to
add it. The rules scatter across your code.

**With transform**

```go
type User struct {
    Name  string `transform:"trim"`
    Email string `transform:"normalize"`

    Address  Address    // tagged fields inside are transformed too
    Contacts []Contact  // so are elements, recursively
}
```

```go
t := transform.New()
t.RegisterString("normalize", normalizeEmail)
t.RegisterString("trim", strings.TrimSpace)
t.Transform(&user)
```

> **Declare the rule next to the field instead of writing and maintaining
> recursive traversal code.**

Add a field, add a tag, done.

## Quick Start

```go
package main

import (
    "fmt"
    "strings"

    "github.com/goxang/transform"
)

type Address struct {
    City string `transform:"trim"`
}

type User struct {
    Name    string  `transform:"trim"`
    Email   string  `transform:"normalize"`
    Address Address
}

func main() {
    t := transform.New()
    t.RegisterString("trim", strings.TrimSpace)
    t.RegisterString("normalize", func(s string) string {
        return strings.ToLower(strings.TrimSpace(s))
    })

    u := User{
        Name:    "  Alice  ",
        Email:   "  ALICE@EXAMPLE.COM ",
        Address: Address{City: "  Tehran "},
    }

    if err := t.Transform(&u); err != nil {
        panic(err)
    }

    fmt.Printf("%+v\n", u)
    // {Name:Alice Email:alice@example.com Address:{City:Tehran}}
}
```

## Installation

```bash
go get github.com/goxang/transform
```

Requires Go 1.19 or newer. Zero dependencies.

## When should I use this?

### Good fit

Use `transform` when:

- the same transformations repeat across structs and DTOs
- structs contain nested values (structs, pointers, slices, maps)
- you want transformation rules colocated with the fields they apply to
- you want to avoid maintaining recursive traversal code by hand
- you want a reusable, name-based set of transformation functions

### Consider a hand-written function when

- the transformation is a couple of assignments on a flat struct
- you are inside an extremely hot loop and every nanosecond counts
- raw throughput matters more than declarative maintainability

## How It Works

`Transform` uses reflection to build an execution plan **once per type**, on
the first call. It reads the type's fields and tags, resolves the registered
functions, and compiles a list of small closures that mutate each field
directly. Fields with nothing to do are dropped from the plan. The plan is
cached in a `sync.Map`.

Later calls run that plan in a simple loop — no reflection, no tag parsing, no
map lookups. After warmup, `Transform` touches your data through the
pre-compiled closures, which is why the hot path stays fast and
allocation-light.

## Registering Transformations

There are no built-in transformations. You register functions under keys, and
a tag value selects the function to run on that field.

```go
t := transform.New()

// string -> string
t.RegisterString("upper", strings.ToUpper)

// string -> (string, error)
t.RegisterStringErr("parse", func(s string) (string, error) {
    return strconv.ParseFloat(s)
})

// any -> (any, error)
t.RegisterAny("double", func(v any) (any, error) {
    n, ok := v.(int)
    if !ok {
        return nil, fmt.Errorf("expected int, got %T", v)
    }
    return n * 2, nil
})
```

| Method | Function signature | Applies to | Notes |
|---|---|---|---|
| `RegisterString` | `func(string) string` | `string` and `*string` fields; `any` fields holding a string | Can't fail |
| `RegisterStringErr` | `func(string) (string, error)` | `string` and `*string` fields; `any` fields holding a string | Error aborts the transform |
| `RegisterAny` | `func(any) (any, error)` | any field type | Escape hatch for non-string types |

A field tagged `transform:"key"` runs the function registered under `"key"`:

```go
type User struct {
    Name  string `transform:"upper"`  // RegisterString
    Count int    `transform:"double"` // RegisterAny
}
```

Details worth knowing:

- A string function on a compound field (struct, slice, map) does not apply —
  it expects a string. The field is still traversed, so inner tagged fields
  are transformed.
- A `RegisterAny` function on a compound field runs on the whole value first,
  then traversal continues into it.
- `RegisterAny` must return a value assignable to the field's type, or
  `Transform` returns an error. Returning `(nil, nil)` leaves the field
  unchanged.
- The whole tag value is one key, used verbatim. `transform:"a,b"` does **not**
  chain two functions; register a key named `"a,b"` or combine both steps in
  one function.

### Custom tag name

```go
t := transform.New(transform.WithTag("xform"))

type User struct {
    Name string `xform:"trim"` // uses "xform" instead of "transform"
}
```

## Nested Structures

`Transform` recurses into nested values of any depth and shape. Untagged
compound fields are visited so the tagged fields inside them still get
transformed.

```go
type Address struct {
    City string `transform:"trim"`
}

type Contact struct {
    Email string `transform:"normalize"`
}

type User struct {
    Name     string             `transform:"trim"`
    Address  *Address           // untagged pointer, visited
    Contacts []*Contact         // untagged slice, each element visited
    Accounts map[string]Contact // untagged map, each value visited
}
```

```go
u := User{
    Name:     " Alice ",
    Address:  &Address{City: " Tehran "},
    Contacts: []*Contact{{Email: " BOB@Example.COM "}},
    Accounts: map[string]Contact{"a": {Email: " CAROL@Example.COM "}},
}
t.Transform(&u)
// u.Name == "Alice"
// u.Address.City == "Tehran"
// u.Contacts[0].Email == "bob@example.com"
// u.Accounts["a"].Email == "carol@example.com"
```

Embedded structs are handled like `encoding/json`: exported fields promoted
from an unexported embedded struct are transformed; an embedded unexported
interface is skipped.

## API

```go
func New(opts ...Option) *Transformer
func WithTag(tag string) Option
```

```go
func (t *Transformer) RegisterString(key string, fn func(string) string)
func (t *Transformer) RegisterStringErr(key string, fn func(string) (string, error))
func (t *Transformer) RegisterAny(key string, fn func(any) (any, error))

func (t *Transformer) Transform(obj any) error
```

- **`New(opts ...Option)`** — create a `Transformer`. The zero value is not
  usable: its `Transform` is a no-op and its `Register` methods panic.
- **`WithTag(tag)`** — set a custom struct-tag name (default `"transform"`).
  Panics on an empty tag.
- **`RegisterString` / `RegisterStringErr` / `RegisterAny`** — register a
  function under a key. Must be called before the first `Transform`; the
  `Transformer` freezes after first use and registering again panics. Keys
  must be non-empty and unique across all three registries.
- **`Transform(obj)`** — apply registered functions to `obj` **in place**.
  `obj` must be a non-nil pointer to a struct, slice, array, or map.

### Error handling

`Transform` stops at the first error and returns it wrapped in a
`*FieldError`:

```go
type FieldError struct {
    Type  string // name of the struct type
    Field string // field that failed
    Key   string // transformation key, if any
    Err   error  // underlying error
}
```

Inspect it with `errors.As`:

```go
var fe *transform.FieldError
if errors.As(err, &fe) {
    fmt.Printf("failed transforming %s.%s: %v\n", fe.Type, fe.Field, fe.Err)
}
```

There is no rollback: fields processed before the error stay transformed.
Passing anything other than a non-nil pointer to a struct, slice, array, or
map returns `transform.ErrInvalidSrc`.

### Concurrency

A `Transformer` is safe for concurrent use once all functions are registered.
After the first `Transform`, registration is frozen and safe to call from
many goroutines.

## Performance

Reflection metadata is cached, so the steady-state cost of `Transform` is a
loop over pre-built closures, not repeated reflection. The first call for a
given type is slower (it builds the plan); every later call is fast.

Measured with `go test -bench=. -benchmem`, Intel Core Ultra 7 265K, Go 1.22:

| struct shape | transform | naive, no cache | json.Unmarshal |
|---|---:|---:|---:|
| 5 fields, 2 transformed | ~173ns | ~321ns | ~1357ns |
| nested struct | ~202ns | ~390ns | ~1232ns |
| 20 fields, 8 transformed | ~324ns | ~1033ns | ~2315ns |

*Naive* is the same logic without the cache — reflection, tag parsing, and
function lookup on every call. *`json.Unmarshal`* is the standard-library
approach people reach for to sanitize data: decode into a fresh struct and
clean afterward. It allocates a new struct and decodes every field, so it is
not an apples-to-apples comparison — it is the cost of the tool people
actually use for this job.

Other shapes:

| benchmark | ns/op | allocs |
|---|---:|---:|
| no transformable fields | ~65 | 1 |
| warm cache, flat struct | ~121 | 2 |
| cold cache (first call per type) | ~1796 | 25 |
| deep nesting, 3 levels | ~89 | 2 |
| slice of 10 structs | ~1500 | 22 |
| map of 4 structs | ~1459 | 20 |
| concurrent, warm | ~56 | 3 |

### vs a hand-written loop

Reflection adds a fixed overhead per call regardless of how much work the
transforms do. That overhead is visible only when transforms are near-free:

| scenario | transform | hand-written | gap |
|---|---:|---:|---:|
| 2 fields, trivial transforms (worst case) | ~90ns | ~15ns | ~6× |
| 3 fields, cleaning user text | ~1.2µs | ~0.9µs | ~1.3× |
| 2 fields, 9 KB strings | ~20.3µs | ~21.1µs | none measurable |
| 50 fields, 20 transformed | ~684ns | ~138ns | ~5× |

The honest reading: cleaning real user input costs roughly a hundred
nanoseconds of framework overhead per call — a rounding error next to the
work itself. Tags only become the expensive part when the transforms are
one-liners on tiny strings inside a hot loop. Write the loop there; everywhere
else, declaring rules next to the data is cheaper to maintain than the loop.

Reproduce locally:

```bash
go test -bench=. -benchmem -count=5 . | tee bench.txt
benchstat bench.txt
```

## Design & Trade-offs

- **Declarative, not imperative.** Rules live next to fields, not in a
  separate walking function.
- **In-place.** No copies, no new structs. Copy first if you need the original.
- **Reflection-based.** This trades a small, fixed runtime overhead for no
  code generation and a simple build. There is a measured overhead on trivial
  transforms; it disappears on real workloads.
- **Frozen after setup.** Like `flag` and `net/http`, a `Transformer` is
  configured once and then read-only, which makes it safe to share across
  goroutines.

## Limitations

- Only exported fields are transformed.
- Map keys are never transformed; only values.
- `[]byte` is not handled as a string; use `RegisterAny` for it.
- Struct and array values inside maps or interfaces are copied, transformed,
  and written back — Go reflection does not allow in-place mutation there.
- Recursive *types* are supported; cyclic *data* is not. A pointer chain that
  loops back on itself recurses without bound.
- Transform functions may not be idempotent: running `Transform` twice applies
  them twice.
- Not a validation framework, not a deep-copy library, and not an object
  mapper — it transforms values in place according to tags.

## Testing

```bash
go test ./...
go test -race ./...
go test -fuzz=. -fuzztime=30s .
```

CI runs tests with `-race -shuffle=on` on Linux, macOS, and Windows across Go
1.19 and the latest release, plus `go vet`, `gofmt`, `golangci-lint`,
`govulncheck`, and short fuzz runs. Coverage is kept at or above 80%.

## Contributing

Bug reports and feature requests are welcome. See the issue templates in
`.github/ISSUE_TEMPLATE` and open an issue before starting larger work.

## License

[MIT](LICENSE)
