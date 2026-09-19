# transform

[![Go Reference](https://pkg.go.dev/badge/github.com/goxang/transform.svg)](https://pkg.go.dev/github.com/goxang/transform)
[![CI](https://github.com/goxang/transform/actions/workflows/ci.yml/badge.svg)](https://github.com/goxang/transform/actions/workflows/ci.yml)
[![CodeQL](https://github.com/goxang/transform/actions/workflows/codeql.yml/badge.svg)](https://github.com/goxang/transform/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/goxang/transform/badge)](https://scorecard.dev/viewer/?uri=github.com/goxang/transform)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Struct-tag driven, in-place field transformation for Go. Register functions by
name, tag fields with those names, and one `Transform` call applies them across
nested structs, pointers, slices, arrays, maps, and interfaces.

Zero dependencies. Go 1.19+.

```bash
go get github.com/goxang/transform
```

## Usage

```go
t := transform.New(transform.WithStrict())
t.RegisterString("trim", strings.TrimSpace)
t.RegisterString("email", func(s string) string {
    return strings.ToLower(strings.TrimSpace(s))
})

type User struct {
    Name  string   `transform:"trim"`
    Email string   `transform:"email"`
    Tags  []string `transform:"trim"`
}

u := User{Name: "  Alice ", Email: " ALICE@Example.COM ", Tags: []string{" go "}}
if err := t.Transform(&u); err != nil {
    return err
}
// {Name:"Alice" Email:"alice@example.com" Tags:["go"]}
```

`Transform` takes a non-nil pointer to a struct, slice, array, or map; anything
else returns `ErrInvalidSrc`. Code already holding a `reflect.Value` can call
`TransformValue` instead.

## Registering

| Method | Function | Applies to |
|---|---|---|
| `RegisterString` | `func(string) string` | strings, `*string`, containers of strings |
| `RegisterStringErr` | `func(string) (string, error)` | same; an error aborts |
| `RegisterBytes` | `func([]byte) []byte` | `[]byte`, `*[]byte`, containers of byte slices |
| `RegisterBytesErr` | `func([]byte) ([]byte, error)` | same; an error aborts |
| `RegisterAny` | `func(any) (any, error)` | any field; result must be assignable, `(nil, nil)` leaves it unchanged |

- A tag value is one opaque key: `transform:"a,b"` is the key `a,b`, not a chain.
- Empty, nil, or duplicate registrations panic.
- `WithTag("xform")` reads `xform:"..."` instead of `transform:"..."`.

## Behavior

- **Strict mode.** By default an unknown key is ignored, so a typo silently does
  nothing. `WithStrict()` reports `ErrUnknownKey` and `ErrUnusableKey` (e.g. a
  string function on an `int`) once per type, at plan build time.
- **Traversal.** Untagged nested values are still walked. Only exported fields
  are transformed, and map keys never are. A `[]byte` is not a string: it needs
  a bytes transform. A `[16]byte` is not a byte slice either — its length is
  part of its type, so it needs `RegisterAny`.
- **Errors.** Stops at the first failure and returns a `*FieldError` with type,
  field, and key. No rollback.
- **Cycles.** Depth is capped at `DefaultMaxDepth` (1000) and returns
  `ErrMaxDepth`; tune with `WithMaxDepth`.
- **Concurrency.** Register first. The first `Transform` freezes the
  `Transformer` (later `Register*` panics); after that it is safe for concurrent
  use. The zero value is unusable, use `New`.
- **Idempotency.** Functions run on every call; running `Transform` twice applies
  them twice.

## Performance

Each type's plan is built on first use and cached, so later calls do no tag
parsing or registry lookups. Intel Core Ultra 7 265K, Go 1.23:

| | ns/op | allocs/op |
|---|---:|---:|
| flat struct, warm | 118 | 2 |
| flat struct, first call | 1930 | 25 |
| 20 fields, 8 transformed | 307 | 5 |
| same, hand-written reflection | 1013 | 24 |
| one `[]byte` field | 73 | 1 |
| same, through `RegisterAny` | 143 | 3 |

Fixed overhead is about 65ns per call versus hand-written code. For trivial
transforms in a hot loop, write the loop.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
