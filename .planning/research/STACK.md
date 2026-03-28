# Stack Research

**Domain:** Go CLI tool — structured text config parsing and format conversion (Cisco MDS NX-OS / Brocade FOS)
**Researched:** 2026-03-28
**Confidence:** HIGH (core stack), MEDIUM (distribution tooling), HIGH (testing)

---

## Recommended Stack

### Core Technologies

| Technology | Version | Purpose | Why Recommended |
|------------|---------|---------|-----------------|
| Go | 1.25.x (current: 1.25.8) | Language runtime | Stated project constraint. Single-binary compilation with `GOOS`/`GOARCH` cross-compilation. No runtime dependencies for ops team distribution. Go 1.24+ added `tool` directive in `go.mod`, simplifying dev toolchain management without `tools.go` hacks. |
| github.com/spf13/cobra | v1.10.2 | CLI framework — subcommands, flags, help | Industry standard for non-trivial Go CLIs. Used by Kubernetes, Hugo, GitHub CLI. Provides nested subcommands (`mds2brocade`, `brocade2mds`), POSIX flag parsing, automatic `--help` generation, and shell autocomplete. Far more structured than `flag` stdlib for a tool with multiple commands and flags. |
| stdlib `bufio` + `regexp` | stdlib (Go 1.25) | Line-by-line config file parsing | NX-OS and FOS configs are custom indentation-structured text — not YAML/TOML/JSON. No third-party grammar parser fits. `bufio.Scanner` reads line-by-line with buffering; `regexp.MustCompile` matches constructs like `device-alias database`, `zone name`, `zoneset activate`. This is the canonical Go approach for vendor CLI config parsing. |
| stdlib `text/template` | stdlib (Go 1.25) | Brocade FOS CLI command output generation | Generates `alicreate`, `zonecreate`, `cfgcreate`, `cfgenable` command blocks from structured internal data. Template-driven output keeps conversion logic separate from formatting, making it trivial to add new output modes (e.g., a script header) without touching converter code. |
| stdlib `log/slog` | stdlib (Go 1.21+, included in Go 1.25) | Structured warnings and conversion diagnostics | Built-in since Go 1.21. Replaces logrus/zerolog for new projects with no external dependency. Supports `WARN` level for "unconvertible construct" messages ops team needs to review. No external dep = no version drift. |

### Supporting Libraries

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/stretchr/testify | v1.11.1 | Test assertions, `require` package | Use `require.Equal`, `require.NoError` in all tests. `require` (not `assert`) stops test execution on first failure — correct behavior for parser unit tests where downstream assertions are meaningless after a parse failure. Use for all table-driven test assertion blocks. |
| github.com/spf13/pflag | v1.0.6 (transitive via cobra) | POSIX flag parsing | Pulled in automatically by cobra. Do not use directly — access flags through cobra's `cmd.Flags()` and `cmd.PersistentFlags()` API. Mentioned here to clarify it is a transitive dep, not an explicit one. |

### Development Tools

| Tool | Purpose | Notes |
|------|---------|-------|
| golangci-lint | v2.11.4 | Static analysis and linting | Use `linters.default: standard` in `.golangci.yml` (v2 config format). Do not enable all linters — start with `standard` preset and add `errcheck`, `govet`, `staticcheck`. Run via `go tool golangci-lint run` using Go 1.24+ tool directive. |
| goreleaser | v2.14.3 | Cross-platform binary release (linux/amd64, darwin/arm64, windows/amd64) | Single `.goreleaser.yml`. Produces GitHub release artifacts + checksums. Ops team installs via `go install` (dev) or downloads pre-built binary (no Go required). |
| cobra-cli | (latest, track via `go tool`) | Scaffolding new cobra commands | Use `go tool cobra-cli add <command>` to scaffold consistent command files. Not a runtime dep. Add via `go get -tool github.com/spf13/cobra-cli`. |

---

## Installation

```bash
# Initialize module
go mod init github.com/<org>/san-conv

# Core runtime dependencies
go get github.com/spf13/cobra@v1.10.2

# Test dependencies
go get github.com/stretchr/testify@v1.11.1

# Dev tool dependencies (Go 1.24+ tool directive — no tools.go needed)
go get -tool github.com/spf13/cobra-cli@latest
go get -tool github.com/golangci/golangci-lint/cmd/golangci-lint@v2.11.4
go get -tool github.com/goreleaser/goreleaser@v2.14.3
```

The resulting `go.mod` tool block:

```
tool (
    github.com/goreleaser/goreleaser
    github.com/golangci/golangci-lint/cmd/golangci-lint
    github.com/spf13/cobra-cli
)
```

Run linter: `go tool golangci-lint run ./...`
Run release: `go tool goreleaser release --snapshot --clean`

---

## Alternatives Considered

| Recommended | Alternative | When to Use Alternative |
|-------------|-------------|-------------------------|
| `bufio` + `regexp` (stdlib) | github.com/tinyinput/confparser | Never for this project. confparser targets IOS switch configs (interfaces, VLANs), has no Brocade support, is minimally maintained (v0.2.0, 2 commits, last update April 2023). Not a fit. |
| `bufio` + `regexp` (stdlib) | Writing a formal grammar with `participle` or `pigeon` | Only if configs were deeply nested or context-sensitive. NX-OS zoning config is line-oriented and regex-friendly. A grammar parser adds significant complexity for marginal gain. |
| cobra | kong (alecthomas/kong) | Kong's struct-tag approach is cleaner for complex nested flag structures. For this tool (2-3 subcommands, handful of flags), cobra's verbosity is manageable and its ecosystem (docs, examples, shell completion) is significantly broader. Choose kong if the command surface grows past ~10 subcommands. |
| cobra | urfave/cli v3 | Viable alternative with simpler API. Switch if cobra's boilerplate becomes friction. urfave/cli v3 has cleaner flag validation. Not recommended here because cobra is the industry default and the team gains from its documentation density. |
| stdlib `log/slog` | github.com/rs/zerolog | Zerolog is faster and has zero allocations — relevant for high-throughput services. For a CLI tool running once per invocation, this performance difference is irrelevant. Adding an external dep for a use case stdlib covers is unjustified. |
| stdlib `log/slog` | github.com/sirupsen/logrus | Logrus is in maintenance-mode as of 2023 — no new features. Do not start new projects with logrus. |
| goreleaser | Manual `go build` + GitHub Actions matrix | Manual approach works but requires per-platform build scripts, manual checksum generation, and no homebrew/scoop integration. goreleaser is the established automation layer; use it. |
| stdlib `text/template` | `fmt.Fprintf` string concatenation | Template approach separates FOS command structure from conversion logic. Concatenation works but becomes unreadable as output formats grow. Templates also allow future multi-format output (JSON inventory, plain script, annotated script) without touching converter code. |

---

## What NOT to Use

| Avoid | Why | Use Instead |
|-------|-----|-------------|
| github.com/sirupsen/logrus | Maintenance-mode since 2023; no new features being accepted | stdlib `log/slog` |
| github.com/tinyinput/confparser | IOS-only, v0.2.0, 2 commits, no Brocade support, last activity 2023 | stdlib `bufio` + `regexp` |
| `go-nxos` (github.com/netascode/go-nxos) | NX-API REST client — for live switch interaction, not offline config parsing | Not applicable; use stdlib parsing |
| ciscoconfparse (Python) | Python, not Go; ops team requires single Go binary with no Python runtime | stdlib parsing in Go |
| Viper (github.com/spf13/viper) | Config management for application settings (env vars, config files, defaults). This tool has no persistent user configuration — inputs are CLI flags and file paths only. Viper adds complexity with no payoff here. | cobra's `cmd.Flags()` directly |
| `encoding/json` for output | FOS CLI commands are not JSON. Using JSON as an intermediate format adds a parsing step for ops team. Output must be pasteable FOS CLI directly. | stdlib `text/template` |
| `html/template` | Escapes characters like `"`, `<`, `>` — will corrupt FOS command output that contains these characters | stdlib `text/template` (the non-HTML variant) |

---

## Stack Patterns by Variant

**If adding a `--dry-run` flag:**

- Write all output to an `io.Writer` interface passed into the converter, not directly to `os.Stdout`
- In normal mode: `converter.Convert(os.Stdout, input)`
- In dry-run mode: `converter.Convert(io.Discard, input)` then print a summary
- Because: `io.Writer` abstraction makes converters trivially testable without capturing stdout

**If adding JSON/machine-readable output (`--output json`):**

- Introduce an internal `ZoningConfig` struct as the canonical intermediate representation
- Keep text/template rendering and JSON marshaling as two separate output adapters
- Because: converters should produce `ZoningConfig`, not text — output format is a concern of the presentation layer

**If distributing via Homebrew:**

- goreleaser handles Homebrew tap generation automatically via `brews:` section in `.goreleaser.yml`
- Requires a separate `homebrew-tap` repository owned by the org

---

## Version Compatibility

| Package | Go Version | Notes |
|---------|------------|-------|
| cobra v1.10.2 | Go 1.18+ | No compatibility concerns with Go 1.25 |
| testify v1.11.1 | Go 1.18+ | Use `require` sub-package, not `assert`, for sequential test correctness |
| golangci-lint v2.x | Go 1.22+ | v2 config format is a breaking change from v1 `.golangci.yml`; use `golangci-lint migrate` if upgrading from v1 |
| goreleaser v2.x | Go 1.21+ | v2 is current; OSS tier sufficient for this project |
| stdlib `log/slog` | Go 1.21+ | Available in Go 1.25; no backport needed |

---

## Sources

- [spf13/cobra releases](https://github.com/spf13/cobra/releases) — v1.10.2 confirmed (December 2024)
- [stretchr/testify releases](https://github.com/stretchr/testify/releases) — v1.11.1 confirmed (August 2025)
- [goreleaser releases](https://github.com/goreleaser/goreleaser/releases) — v2.14.3 confirmed (March 2026)
- [golangci-lint releases](https://github.com/golangci/golangci-lint/releases) — v2.11.4 confirmed (March 2026)
- [Go 1.24 Release Notes](https://go.dev/doc/go1.24) — `tool` directive in `go.mod`, verified official
- [Go 1.25 Release](https://go.dev/blog/go1.25) — Go 1.25.8 is current stable (March 2026)
- [bufio package docs](https://pkg.go.dev/bufio) — stdlib line-by-line parsing, HIGH confidence
- [text/template package docs](https://pkg.go.dev/text/template) — stdlib template generation, HIGH confidence
- [slog structured logging](https://go.dev/blog/slog) — stdlib since Go 1.21, official announcement
- [golangci-lint v2 migration](https://ldez.github.io/blog/2025/03/23/golangci-lint-v2/) — v2 config format changes, MEDIUM confidence (third-party blog, consistent with official docs)
- [Go logging landscape 2025](https://uptrace.dev/blog/golang-logging) — logrus maintenance-mode confirmed, MEDIUM confidence
- [tinyinput/confparser](https://github.com/tinyinput/confparser) — v0.2.0, IOS-only, last commit 2023, verified NOT suitable
- WebSearch: cobra vs kong vs urfave/cli comparison, MEDIUM confidence (multiple sources agree)
- WebSearch: goreleaser distribution workflow, MEDIUM confidence (verified against official goreleaser.com)

---

*Stack research for: Go CLI tool — SAN zoning config converter (Cisco MDS NX-OS / Brocade FOS)*
*Researched: 2026-03-28*
