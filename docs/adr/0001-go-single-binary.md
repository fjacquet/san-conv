# ADR-0001: Go as implementation language

**Date:** 2026-03-28
**Status:** Accepted

## Context

The tool is built for an ops team managing SAN fabrics. They need to run it on workstations, jump servers, and sometimes automation hosts — across Linux, macOS, and Windows. Installing a runtime (Python, Node, JVM) is a friction point in ops environments with strict change control. The tool is a one-shot CLI invoked per migration, not a long-running service.

## Decision

Implement in Go. Produce a single statically-linked binary per platform using `GOOS`/`GOARCH` cross-compilation via goreleaser.

## Rationale

- **Single binary**: `go build` produces a self-contained executable with zero runtime dependencies. Ops team installs via `go install` or downloads a release binary.
- **Cross-compilation**: goreleaser produces linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 from a single CI job.
- **stdlib sufficiency**: Config parsing (`bufio` + `regexp`), templated output (`text/template`), and structured logging (`log/slog`) are all in stdlib. No dependency graph to audit.
- **Fast startup**: CLI tools need sub-100ms startup. Go's startup time is negligible; Python imports would add 200-500ms.

## Alternatives considered

| Alternative | Rejected because |
|-------------|-----------------|
| Python | Requires Python ≥ 3.10 installed; virtualenv management; no single-binary distribution without PyInstaller (adds 30MB+) |
| Rust | Longer dev cycle for a config-translation tool; no stdlib regex (requires crate); team familiarity lower |
| Shell script | Not cross-platform (Windows); regex handling fragile; no structured testing |

## Consequences

- Go 1.21+ required in CI (for `log/slog`); go.mod pins minimum version
- Dev toolchain (golangci-lint, goreleaser) managed via `go tool` directive (Go 1.24+)
- Binary size ~6MB stripped; acceptable for a CLI tool
