# Security Policy

## Supported versions

The latest minor release is supported. This package has no dependencies, so its
attack surface is the standard library plus its own reflection code.

## Reporting a vulnerability

Report privately through [GitHub Security
Advisories](https://github.com/goxang/transform/security/advisories/new) rather
than a public issue.

Please include the affected version, a minimal reproducer, and what an attacker
gains. You can expect an acknowledgement within a week.

## Disclosure

Vulnerabilities are handled through coordinated disclosure. A fix is released
within 90 days of the report, and the advisory is published with the release
that fixes it, crediting the reporter unless they ask otherwise.

## Verifying a release

Each release attaches a source archive signed with Sigstore:

```bash
cosign verify-blob \
  --bundle transform-vX.Y.Z.tar.gz.sigstore.json \
  --certificate-identity-regexp '^https://github.com/goxang/transform/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  transform-vX.Y.Z.tar.gz
```

## Scope

Realistic issues for a package like this one:

- Input that makes `Transform` panic, or that reaches a stack overflow the
  depth limit was supposed to bound.
- A traversal path that writes to memory it should not, or that transforms a
  field the documented rules say it must leave alone.
- A data race in the plan cache or the freeze protocol.

Out of scope: the behavior of transformation functions you register — those are
your code, and `Transform` calls them exactly as given.
