# transform

[![Go Reference](https://pkg.go.dev/badge/github.com/goxang/transform.svg)](https://pkg.go.dev/github.com/goxang/transform)

Struct-tag driven field transformation for Go. You register functions under
names, tag fields with those names, and a single `Transform` call walks the
whole value — nested structs, pointers, slices, arrays, maps, interfaces — and
mutates it in place.

The point is to keep normalization rules (trim, lowercase, redact, ...) next to
the fields they apply to, instead of in a recursive walking function you have to
remember to update every time the struct grows a field.

## Install

```bash
go get github.com/goxang/transform
```

Go 1.19 or newer. No dependencies, no code generation.

## Usage

```go
t := transform.New()
t.RegisterString("trim", strings.TrimSpace)
t.RegisterString("email", func(s string) string {
    return strings.ToLower(strings.TrimSpace(s))
})

type User struct {
    Name  string `transform:"trim"`
    Email string `transform:"email"`
    Age   int
}

u := User{Name: "  Alice ", Email: " ALICE@Example.COM ", Age: 30}
if err := t.Transform(&u); err != nil {
    return err
}
// u.Name  == "Alice"
// u.Email == "alice@example.com"
// u.Age   == 30, untagged and untouched
```

`Transform` needs a non-nil pointer to a struct, slice, array, or map. Anything
else returns `ErrInvalidSrc`.

## Registering functions

The package ships no transformations of its own. Three registration methods,
picked by the shape of your function:

| Method | Signature | Applies to |
|---|---|---|
| `RegisterString` | `func(string) string` | `string` / `*string` fields, and `any` fields holding a string |
| `RegisterStringErr` | `func(string) (string, error)` | same, but an error aborts the transform |
| `RegisterAny` | `func(any) (any, error)` | any field type — the escape hatch |

```go
t.RegisterAny("round", func(v any) (any, error) {
    f, ok := v.(float64)
    if !ok {
        return nil, fmt.Errorf("expected float64, got %T", v)
    }
    return math.Round(f*100) / 100, nil // must be assignable to the field
})

type Order struct {
    Total float64 `transform:"round"`
}
```

Things worth knowing before you reach for the tag:

- The tag value is one opaque key, looked up verbatim. `transform:"a,b"` does
  **not** chain two functions; register a key literally named `"a,b"`, or do
  both steps in one function.
- Keys are unique across all three registries. An empty key, a nil function, or
  a duplicate key panics at registration.
- A string function on a compound field (struct, slice, map) does not apply —
  the field is still traversed, so tagged fields inside it are transformed.
- A `RegisterAny` function on a compound field runs on the whole value first,
  then traversal continues into it.
- `RegisterAny` must return something assignable to the field's type or
  `Transform` fails; returning `(nil, nil)` leaves the field unchanged.

Use `transform.New(transform.WithTag("xform"))` to read `xform:"..."` tags
instead of the default `transform:"..."`.

## Traversal

Untagged compound fields are still visited, so tagged fields nested inside them
get transformed:

```go
type Address struct{ City string `transform:"trim"` }
type Contact struct{ Email string `transform:"email"` }

type User struct {
    Name     string `transform:"trim"`
    Address  *Address           // visited
    Contacts []*Contact         // every element visited
    Accounts map[string]Contact // every value visited
}
```

Details:

- Only exported fields are transformed. Embedded structs behave like
  `encoding/json`: exported fields promoted through an unexported embedded
  struct are transformed; an embedded unexported interface is skipped.
- Map **keys** are never transformed, only values.
- `[]byte` is not treated as a string. Use `RegisterAny` for it.
- Struct and array values reached through a map or an interface are not
  addressable, so they are copied, transformed, and written back.
- Recursive types work, including mutually recursive ones. Cyclic *data* does
  not: traversal follows pointers, so a pointer chain that loops back on itself
  recurses without bound.

## Errors

`Transform` stops at the first error and wraps it in a `*FieldError` carrying
the struct type, field name, and transformation key:

```go
var fe *transform.FieldError
if errors.As(err, &fe) {
    log.Printf("%s.%s (key %q): %v", fe.Type, fe.Field, fe.Key, fe.Err)
}
```

There is no rollback. Fields transformed before the failure stay transformed.

## Lifecycle and concurrency

A `Transformer` is configured once and then read-only. The first `Transform`
call freezes it; any `Register*` call after that panics. Once frozen it is safe
to share across goroutines — the per-type plan cache is a `sync.Map` and the
transform path only reads the registries.

The zero value is not usable: use `New`.

## Performance

The first `Transform` of a given type builds an execution plan — reads fields
and tags, resolves the registered functions, compiles one closure per field, and
drops the fields with nothing to do — then caches it by `reflect.Type`. Every
later call is a loop over those closures, with no tag parsing and no registry
lookups.

AMD Ryzen 7 4800H, Go 1.26, `go test -bench=. -benchmem`:

| | ns/op | allocs/op |
|---|---:|---:|
| flat struct, warm cache | 355 | 2 |
| flat struct, first call for the type | 4339 | 16 |
| 20 fields, 8 transformed | 720 | 5 |
| same 20 fields, uncached reflection | 2305 | 5 |

Against a hand-written equivalent, the fixed overhead is roughly 150–200ns per
call. That is the whole story: on two trivial transforms over short strings it
is a ~9× slowdown (201ns vs 22ns), on three realistic text-cleaning functions it
disappears into the noise (1.7µs vs 1.9µs), and on 9 KB strings it is
unmeasurable. If your transforms are one-liners inside a hot loop, write the
loop. Otherwise the tag is cheaper to maintain than the traversal code.

Reproduce with:

```bash
go test -bench=. -benchmem -count=5 . | tee bench.txt && benchstat bench.txt
```

## What it is not

Not a validator, not a deep-copy library, not an object mapper. It mutates a
value in place according to tags, and nothing else. Transform functions are
applied every call, so running `Transform` twice applies them twice — write
idempotent functions if that matters.

## Development

```bash
go test ./...
go test -race ./...
go test -fuzz='^FuzzTransform$' -run='^$' -fuzztime=30s .
```

CI runs the suite with `-race -shuffle=on` on Linux, macOS, and Windows against
Go 1.19 and the current release, plus `go vet`, `gofmt`, `golangci-lint`,
`govulncheck`, and short fuzz runs.

Bug reports and feature requests are welcome — see the issue templates in
`.github/ISSUE_TEMPLATE`, and open an issue before starting larger work.

## License

[MIT](LICENSE)
