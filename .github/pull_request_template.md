## What this changes

<!-- One or two sentences. Link the issue this closes, if any. -->

## Why

<!-- What was wrong or missing. For a bug, what the old behavior was. -->

## Checklist

- [ ] Tests cover the new behavior (and fail without the change)
- [ ] `go test -race -shuffle=on ./...` passes
- [ ] `go vet ./...` and `gofmt -l .` are clean
- [ ] Public API changes have doc comments and a `CHANGELOG.md` entry
- [ ] Still builds on Go 1.19 (or the minimum is raised deliberately)
- [ ] Hot-path changes include before/after benchmarks below

## Benchmarks

<!-- Delete if not applicable. State CPU and Go version. -->
