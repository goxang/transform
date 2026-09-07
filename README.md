# transform

[![Go Reference](https://pkg.go.dev/badge/github.com/goxang/transform.svg)](https://pkg.go.dev/github.com/goxang/transform)
[![CI](https://github.com/goxang/transform/actions/workflows/ci.yml/badge.svg)](https://github.com/goxang/transform/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/goxang/transform)](https://goreportcard.com/report/github.com/goxang/transform)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![CodeQL](https://github.com/goxang/transform/actions/workflows/codeql.yml/badge.svg)](https://github.com/goxang/transform/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/goxang/transform/badge)](https://scorecard.dev/viewer/?uri=github.com/goxang/transform)

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
    Name  string   `transform:"trim"`
    Email string   `transform:"email"`
    Tags  []string `transform:"trim"`
    Age   int
}

u := User{Name: "  Alice ", Email: " ALICE@Example.COM ", Tags: []string{" go "}, Age: 30}
if err := t.Transform(&u); err != nil {
    return err
}
// u.Name  == "Alice"
// u.Email == "alice@example.com"
// u.Tags  == []string{"go"}
// u.Age   == 30, untagged and untouched
```

`Transform` needs a non-nil pointer to a struct, slice, array, or map. Anything
else returns `ErrInvalidSrc`.

## Registering functions

The package ships no transformations of its own. Three registration methods,
picked by the shape of your function:

| Method | Signature | Applies to |
|---|---|---|
| `RegisterString` | `func(string) string` | `string` / `*string` fields, containers of strings, and `any` fields holding a string |
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
  **not** chain two functions, and there are no json-style options — register a
  key literally named `"a,b"`, or do both steps in one function.
- Keys are unique across all three registries. An empty key, a nil function, or
  a duplicate key panics at registration.
- A string function applies to a container whose elements bottom out in strings:
  `[]string`, `map[string]string`, `[3]string`, `[][]string`, `map[K]*string`.
  Every element is transformed; map **keys** never are.
- A string function on any other compound field (a struct, `[]int`) does not
  apply — the field is still traversed, so tagged fields inside it are
  transformed.
- A `RegisterAny` function on a compound field runs on the whole value first,
  then traversal continues into it.
- `RegisterAny` must return something assignable to the field's type or
  `Transform` fails; returning `(nil, nil)` leaves the field unchanged.

Use `transform.New(transform.WithTag("xform"))` to read `xform:"..."` tags
instead of the default `transform:"..."`.

## Strict mode

By default a tag that resolves to nothing is ignored. That makes a typo look
exactly like a working transformation:

```go
type User struct {
    Name string `transform:"uppr"` // silently does nothing
}
```

`WithStrict` turns those into errors, checked once when the type's plan is
built, so it costs nothing on the hot path:

```go
t := transform.New(transform.WithStrict())
t.RegisterString("upper", strings.ToUpper)

err := t.Transform(&User{})
// transform User.Name (key "uppr"): no transformation registered for key
errors.Is(err, transform.ErrUnknownKey) // true
```

It reports `ErrUnknownKey` for a key that was never registered (including a
`transform:"upper,omitempty"` tag, which is one key named `upper,omitempty`),
and `ErrUnusableKey` for a key that cannot act on the field it is attached to —
a string function on an `int`, or any tag on an unexported field.

Strict mode is recommended for new code. It is opt-in so it cannot break an
existing program on upgrade.

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
- Recursive *types* work, including mutually recursive ones. Cyclic *data* is
  bounded — see below.

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

The sentinel errors are `ErrInvalidSrc`, `ErrMaxDepth`, `ErrUnknownKey`, and
`ErrUnusableKey`; match them with `errors.Is`.

## Cyclic data

Traversal follows pointers, slices, maps, and interfaces, so a value graph that
loops back on itself has no natural end. Rather than recursing into a stack
overflow — which Go cannot recover from — traversal is bounded at
`DefaultMaxDepth` (1000) levels and returns `ErrMaxDepth`:

```go
n := &Node{Name: "loop"}
n.Next = n
errors.Is(t.Transform(n), transform.ErrMaxDepth) // true
```

`transform.New(transform.WithMaxDepth(50))` tightens the bound; raise it if you
have legitimately deeper data. The check is two integer comparisons per struct,
so it does not show up in the benchmarks.

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

Intel Core Ultra 7 265K, Go 1.23, `go test -bench=. -benchmem -count=6`,
median of six:

| | ns/op | allocs/op |
|---|---:|---:|
| flat struct, warm cache | 118 | 2 |
| flat struct, first call for the type | 1930 | 25 |
| 20 fields, 8 transformed | 307 | 5 |
| same 20 fields, hand-written reflection walk | 1013 | 24 |
| 4-entry `map[string]Struct` | 1120 | 13 |

Against a hand-written non-reflective equivalent, the fixed overhead is roughly
65ns per call. That is the whole story: on two trivial transforms over short
strings it is a ~5× slowdown (81ns vs 15ns), on three realistic text-cleaning
functions it is ~20% (1.05µs vs 0.87µs), and on 9 KB strings it is unmeasurable
(19.5µs either way). If your transforms are one-liners inside a hot loop, write
the loop. Otherwise the tag is cheaper to maintain than the traversal code.

Reproduce with:

```bash
go test -bench=. -benchmem -count=6 . | tee bench.txt && benchstat bench.txt
```

## What it is not

Not a validator, not a deep-copy library, not an object mapper. It mutates a
value in place according to tags, and nothing else. Transform functions are
applied every call, so running `Transform` twice applies them twice — write
idempotent functions if that matters.

## Development

Every check CI runs has a make target, so a red build reproduces locally
without reading a workflow file:

```bash
make            # list the targets
make all        # tidy-check, fmt-check, vet, test, lint, coverage
make test       # -race -shuffle=on
make fuzz       # every fuzz target, 30s each (FUZZTIME=2m to go longer)
make go-benchmark-compare   # benchmarks against origin/main
```

CI is split into the same jobs:

| Job | What it enforces |
|---|---|
| `test` | suite with `-race -shuffle=on` on Linux, macOS, and Windows against Go 1.19 and the current release, plus vet, gofmt, a benchmark smoke run, and 10s of every fuzz target |
| `lint` | `golangci-lint`, version and enabled set pinned in `.golangci.yml` |
| `tidy-check` | `go mod tidy` and `gofmt` are no-ops on the committed tree |
| `coverage-test` | 85% line coverage floor |
| `go-benchmark-test` | on a pull request, benchmarks against the base branch; an increase in allocations per operation fails the job |
| `vuln` | `govulncheck` |
| `codeql` | GitHub's Go queries, on every change and weekly |
| `osv-scanner` | dependency advisories, on every change and weekly |
| `license-scan` | dependency licenses against a permissive allowlist |
| `scorecard` | OpenSSF supply-chain posture, weekly |
| `tag` / `release` | on main only: tags the version the changelog declares and publishes the release |

Benchmark timings on a shared runner move by double digits between runs, so
the comparison job reports them and gates only on allocation counts, which are
deterministic.

Releasing is a merge. When a pull request lands on main and every job above
passes, the `tag` job reads the newest version in `CHANGELOG.md`, and if no tag
exists for it, creates one and publishes the release from that section. A merge
that does not add a version section releases nothing.

Every action is pinned to a commit SHA; dependabot proposes the bumps.

See [CONTRIBUTING.md](CONTRIBUTING.md). Bug reports and feature requests are
welcome — see the issue templates in `.github/ISSUE_TEMPLATE`, and open an issue
before starting larger work.

## License

[MIT](LICENSE)
