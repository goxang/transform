# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `TransformValue(reflect.Value)` transforms a value a caller already holds as
  a `reflect.Value`, without boxing it back into an interface for `Transform`.
  It follows pointers and interfaces, transforms a struct, slice, array, or map
  at the end of that chain, and treats anything else as nothing to do rather
  than as `ErrInvalidSrc`. Unaddressable structs and arrays are left unchanged,
  matching what passing them by value would do. On an Intel Ultra 7 265K with
  Go 1.23: 16.0ns -> 11.3ns for a struct with no transformable fields, 124ns ->
  110ns for a flat struct with two, no change in allocations.

### Fixed

- Writing an element back into an unaddressable array no longer panics. The
  case was unreachable through `Transform`, which requires a pointer, but
  `TransformValue` can be handed one.

## [0.2.0] - 2026-09-07

### Added

- String transforms now apply to containers whose elements bottom out in
  strings: `[]string`, `map[K]string`, `[N]string`, `map[K]*string`,
  `[][]string`, and any nesting of those. Previously a `transform:"upper"` tag
  on a `[]string` field was silently a no-op.
- `WithStrict()` makes an unresolvable tag an error instead of a silent no-op.
  It reports `ErrUnknownKey` for a key that was never registered — which
  includes json-style tag options such as `transform:"upper,omitempty"` — and
  `ErrUnusableKey` for a registered key that cannot act on the field it is
  attached to, including any tag on an unexported field.
- `WithMaxDepth(n)` and `DefaultMaxDepth` bound traversal depth. Cyclic data
  now returns `ErrMaxDepth` rather than recursing until the stack overflows,
  which Go cannot recover from.
- New sentinel errors: `ErrMaxDepth`, `ErrUnknownKey`, `ErrUnusableKey`.
- Runnable examples for string containers, strict mode, and the depth limit;
  a fuzz target for the container path.

### Changed

- Map traversal reuses one key buffer and one value buffer per map instead of
  allocating per entry: ~24% faster and ~35% fewer allocations on a 4-entry
  `map[string]Struct` (1377ns/20 allocs → 1091ns/13 allocs).
- Compiled field closures carry the element type's plan, so walking a slice or
  map field no longer does a type-cache lookup on every call.
- Concurrent first-use of the same type now converges on a single cached plan
  (`LoadOrStore`) instead of leaving each goroutine on its own copy.

### Fixed

- A `Transform` on cyclic data no longer crashes the process with a stack
  overflow.

### Repository

- Make targets covering every check CI runs, so a red build reproduces locally.
- CI split into named jobs: `lint`, `tidy-check`, `test`, `coverage-test`,
  `go-benchmark-test`, and `vuln`.
- `go-benchmark-test` compares a pull request's benchmarks against the base
  branch. Allocation counts are deterministic, so an increase fails the job;
  wall-clock deltas are reported but never gated.
- `tidy-check` fails if `go mod tidy` or `gofmt` would change the committed
  tree.
- New workflows: CodeQL, OSV-Scanner, license scan against a permissive
  allowlist, OpenSSF Scorecard, and a stale-issue sweep.
- Every GitHub Action is pinned to a commit SHA; dependabot proposes the bumps.
- Releases are cut by merging to `main`: once every CI job passes there, the
  `tag` job tags the newest version in this file, if it is not tagged already,
  and the release job publishes it from that section.

## [0.1.0] - 2026-09-05

### Added

- Initial release: tag-driven in-place struct field transformation with a
  cached per-type execution plan and a dynamic function registry.

[Unreleased]: https://github.com/goxang/transform/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/goxang/transform/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/goxang/transform/releases/tag/v0.1.0
