# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 1.x     | ✅ Yes     |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Report security issues privately via [GitHub Security Advisories](https://github.com/fjacquet/san-conv/security/advisories/new).

Include:
- Description of the vulnerability
- Steps to reproduce
- Affected versions
- Suggested fix (if any)

You will receive a response within 7 days. If the issue is confirmed, a patch will be released as soon as possible.

## Scope

`san-conv` is an **offline, static-analysis CLI tool**. It reads config files from disk and writes text output. It has no network access, no web server, no database, and no authentication surface.

The primary security concern is the safety of **input file parsing** — malicious input files should not cause crashes, infinite loops, or out-of-memory conditions. ReDoS-style attacks on config parsing regexes are in scope.

## Known Limitations

- `github.com/docker/docker` (transitive dep from `golangci-lint`, dev-only) has open CVEs pending a v29 release. This package is **not included in the distributed binary** — only in the development toolchain.
