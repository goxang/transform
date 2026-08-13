# transform

[![Go Reference](https://pkg.go.dev/badge/github.com/MohammadmahdiAhmadi/transform.svg)](https://pkg.go.dev/github.com/MohammadmahdiAhmadi/transform)
[![CI](https://github.com/MohammadmahdiAhmadi/transform/actions/workflows/ci.yml/badge.svg)](https://github.com/MohammadmahdiAhmadi/transform/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-84.8%25-brightgreen)](https://github.com/MohammadmahdiAhmadi/transform/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Use struct tags to define how fields should change. One call applies your
functions to the whole value, including nested structs, pointers, slices,
maps, and interfaces.

```go
t := transform.New()
t.RegisterString("normalize", func(s string) string {
    return strings.ToLower(strings.TrimSpace(s))
})

type User struct {
    Email string `transform:"normalize"`
}

u := User{Email: "  ALICE@Example.COM "}
t.Transform(&u) // u.Email == "alice@example.com"
```

Common uses: cleaning user input before saving it, hiding secrets before
logging, normalizing values at API boundaries.

## Install

```bash
go get github.com/MohammadmahdiAhmadi/transform
```

Go 1.19 or newer. No dependencies.

## API

```go
t := transform.New()                  // optional: transform.New(transform.WithTag("custom"))
t.RegisterString("key", func(s string) string)              // string -> string
t.RegisterStringErr("key", func(s string) (string, error))  // string -> (string, error)
t.RegisterAny("key", func(v any) (any, error))              // any -> (any, error)

err := t.Transform(&obj)   // changes obj in place, recursively
```

`Transform` accepts a non-nil pointer to a struct, slice, array, or map.
It matches fields to functions by the tag value: a field tagged
`transform:"key"` runs the function registered under `"key"`. Fields
without a tag, and tags with no registered function, are left unchanged.

If a transform function returns an error, `Transform` stops and returns
that error wrapped in a `*FieldError`. The wrapper holds the struct type,
field name, and tag key. Inspect it with `errors.As`.

Register all functions before the first `Transform` call. After the
first call the `Transformer` becomes read-only, and registering again
panics — the same idea as `flag` or `net/http`. Once set up, you can call
`Transform` from many goroutines at the same time.

## Features

- Visits everything: structs, pointers, slices, arrays, maps, interfaces
- Reflection is used once per type, then never again. After warmup, calls run pre-built closures in a simple loop: no tag parsing, no map lookups, no `Kind()` checks
- Nearly all allocations on the hot path are the strings your functions produce
- Custom tag name via `WithTag`
- Zero dependencies

## Notes

**Changes the original, makes no copy.**
`Transform` changes the value you pass in. Pass a pointer; passing a
plain value returns `ErrInvalidSrc`. Copy the value first if you want to
keep the original.

**Nested values are always visited.**
A struct, slice, array, or map field is visited even when it has no tag,
so the tagged fields inside it still get transformed. If the field itself
has a tag, the `RegisterAny` function runs on the whole value first, then
the visit continues inside the result. String functions skip compound
fields (they expect strings), but inner tagged fields are still visited.

```go
type Inner struct {
    Email string `transform:"lower"`
}
type Outer struct {
    Inner                      // no tag: still visited, Email gets "lower"
    Other Inner `transform:"stamp"` // "stamp" runs first, then Email gets "lower"
}
```

**One field, one function.**
The whole tag value is used as one key, exactly as written.
`transform:"a,b"` does not chain two functions. If you want both, combine
them inside one function.

**Unexported fields are skipped.**
Fields that start with a lowercase letter never get transformed. One
exception: when an unexported struct is embedded, Go promotes its
exported fields, so those are transformed too (same behavior as
`encoding/json`). Embedded unexported interfaces are skipped.

**`RegisterAny` must return the right type.**
The returned value must fit the field type, or `Transform` fails.
Returning `(nil, nil)` leaves the field unchanged.

**No rollback.**
`Transform` stops at the first error. Fields processed before the error
stay transformed. Map values are visited in random order, so when several
map entries fail, the reported error can differ between runs.

**Map keys never change.** Only map values are transformed. `[]byte` is
not treated as a string; use `RegisterAny` for it.

## How it works

Reflection is used once per type, on the first call only. The library
reads the type's fields and tags, resolves the functions, and compiles a
plan of small closures for that type. It stores the plan in a `sync.Map`.
Fields that need no work are dropped at that point, so later calls are a
simple loop over closures — no reflection, no tag parsing, no map
lookups. This is why the hot path stays fast: after warmup, `Transform`
touches your data directly.

Recursive types are supported, including types that reference each
other. But the actual data must not contain cycles: if a pointer chain
loops back to an earlier value, `Transform` will never finish. Cut such
links before transforming.

## Performance

Warm path on a flat 5-field struct: about 160ns and 3 allocs per call,
almost all of it the strings the transform functions produce. The first
call for a type costs roughly 1.8µs to build its plan; that happens once.

Two baselines for comparison. **Naive** is the same logic without the
cache: reflection, tag parsing, and function lookup on every call.
**`json.Unmarshal`** is the standard-library way to sanitize data — decode
into a fresh struct and clean afterwards. It allocates a whole new struct
and decodes every field, so it is not an apples-to-apples comparison; it
is the cost of the tool people actually reach for.

Go 1.23, Core Ultra 7 265K:

| struct shape | transform | naive, no cache | json.Unmarshal |
|---|---:|---:|---:|
| 5 fields, 2 transformed | 164ns | 301ns | 883ns |
| nested struct | 190ns | 369ns | 1198ns |
| 20 fields, 8 transformed | 293ns | 1065ns | 2235ns |

Other shapes:

| benchmark | ns/op | allocs |
|---|---:|---:|
| no transformable fields | 62 | 1 |
| warm cache, flat struct | 119 | 2 |
| cold cache (first call per type) | 1776 | 25 |
| deep nesting, 3 levels | 90 | 2 |
| slice of 10 structs | 1390 | 22 |
| map of 4 structs | 1297 | 20 |
| concurrent, warm | 43 | 3 |

If you always transform the same known struct in a hot loop, a
hand-written loop will be faster. This library gives you less code and
one place to define the rules; it does not promise reflection-free speed.

```bash
go test -bench=. -benchmem -count=5 . | tee bench.txt
benchstat bench.txt
```

## Limitations

- Only exported fields are transformed.
- Map keys are never transformed.
- `[]byte` is not handled as a string; use `RegisterAny`.
- Struct and array values inside maps or interfaces are copied, transformed, and written back. Go reflection does not let you change them directly there.
- Recursive *types* are fine; cyclic *data* is not.
- Transform functions may not be idempotent: running `Transform` twice applies them twice.

## Testing

```bash
go test ./...
go test -race ./...
go test -fuzz=. -fuzztime=30s .
```

## License

[MIT](LICENSE)
