# Contributing

Thanks for taking the time. This is a small, dependency-free package; the bar
for new surface area is high and the bar for correctness is higher.

## Before you start

Open an issue first for anything beyond a bug fix or a doc correction. The
[issue templates](.github/ISSUE_TEMPLATE) ask the questions that usually decide
whether a change belongs here.

Changes that are likely to be accepted:

- A reproducible bug with a failing test.
- A traversal case the package gets wrong (unusual field shapes, addressability,
  embedding).
- Documentation that clarifies behavior someone got wrong in practice.

Changes that are likely to be declined:

- New registration signatures. The three that exist cover the space; a fourth
  has to earn its place against `RegisterAny`.
- Tag syntax — options, chaining, conditionals. The tag value is one opaque key
  on purpose.
- Anything that turns this into a validator, mapper, or copier.
- New dependencies. The package has none and intends to keep it that way.

## Working on a change

```bash
go test ./...                # unit tests and examples
go test -race -shuffle=on ./...
go vet ./...
gofmt -l .                   # must print nothing
go test -bench=. -benchmem . # if you touched the hot path
```

Fuzz targets, if you touched traversal:

```bash
go test -fuzz='^FuzzTransform$' -run='^$' -fuzztime=30s .
```

Requirements for a pull request:

- Tests for the behavior you changed. Coverage is gated at 85%.
- The module builds on Go 1.19 — no newer standard-library APIs without raising
  the minimum in `go.mod`, which is a deliberate decision, not a side effect.
- Public API changes come with doc comments and a `CHANGELOG.md` entry under
  "Unreleased".
- Benchmarks before/after for hot-path changes, with the machine and Go version
  stated.

## Commit messages

[Conventional Commits](https://www.conventionalcommits.org/): `fix:`, `feat:`,
`docs:`, `perf:`, `test:`, `refactor:`, `chore:`. Breaking changes get a `!`
and a `BREAKING CHANGE:` footer.

## Releasing

Maintainers only:

1. Update `CHANGELOG.md` — move "Unreleased" to the new version with a date.
2. Tag `vX.Y.Z` on `main` and push it. The release workflow verifies the tag
   matches the changelog, runs the full suite, and publishes the release notes.
3. `GOPROXY=proxy.golang.org go list -m github.com/goxang/transform@vX.Y.Z` to
   warm the module proxy.
