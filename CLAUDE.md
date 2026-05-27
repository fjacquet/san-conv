<!-- GSD:project-start source:PROJECT.md -->
## Project

**san-conv — SAN Zoning Config Converter**

A Go CLI tool that converts SAN fabric zoning configurations between Cisco MDS (NX-OS) and Brocade FOS formats. The primary use case is migrating zoning from Cisco MDS to Brocade switches, with bidirectional conversion supported. It is built for ops teams who need a reliable, distributable binary with no runtime dependencies.

**Core Value:** Given a full MDS running-config file, produce correct, ready-to-apply Brocade FOS CLI commands and a runnable script — with warnings for anything that couldn't be converted cleanly.

### Constraints

- **Tech stack**: Go — single binary, no runtime deps, easy to distribute to ops team
- **Error handling**: Warn and continue — partial output is better than stopping mid-conversion
- **Input**: Full config file (not live switch connection) — tool is offline/static analysis only
- **Compatibility**: Must handle real-world MDS configs including edge cases (empty zones, comments, long names)
<!-- GSD:project-end -->

<!-- GSD:stack-start source:research/STACK.md -->
## Technology Stack

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
## Installation
# Initialize module
# Core runtime dependencies
# Test dependencies
# Dev tool dependencies (Go 1.24+ tool directive — no tools.go needed)
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
## Stack Patterns by Variant
- Write all output to an `io.Writer` interface passed into the converter, not directly to `os.Stdout`
- In normal mode: `converter.Convert(os.Stdout, input)`
- In dry-run mode: `converter.Convert(io.Discard, input)` then print a summary
- Because: `io.Writer` abstraction makes converters trivially testable without capturing stdout
- Introduce an internal `ZoningConfig` struct as the canonical intermediate representation
- Keep text/template rendering and JSON marshaling as two separate output adapters
- Because: converters should produce `ZoningConfig`, not text — output format is a concern of the presentation layer
- goreleaser handles Homebrew tap generation automatically via `brews:` section in `.goreleaser.yml`
- Requires a separate `homebrew-tap` repository owned by the org
## Version Compatibility
| Package | Go Version | Notes |
|---------|------------|-------|
| cobra v1.10.2 | Go 1.18+ | No compatibility concerns with Go 1.25 |
| testify v1.11.1 | Go 1.18+ | Use `require` sub-package, not `assert`, for sequential test correctness |
| golangci-lint v2.x | Go 1.22+ | v2 config format is a breaking change from v1 `.golangci.yml`; use `golangci-lint migrate` if upgrading from v1 |
| goreleaser v2.x | Go 1.21+ | v2 is current; OSS tier sufficient for this project |
| stdlib `log/slog` | Go 1.21+ | Available in Go 1.25; no backport needed |
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
<!-- GSD:stack-end -->

<!-- GSD:conventions-start source:CONVENTIONS.md -->
## Conventions

Conventions not yet established. Will populate as patterns emerge during development.
<!-- GSD:conventions-end -->

<!-- GSD:architecture-start source:ARCHITECTURE.md -->
## Architecture

Architecture not yet mapped. Follow existing patterns found in the codebase.
<!-- GSD:architecture-end -->

<!-- GSD:workflow-start source:GSD defaults -->
## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.
<!-- GSD:workflow-end -->



<!-- GSD:profile-start -->
## Developer Profile

> Profile not yet configured. Run `/gsd:profile-user` to generate your developer profile.
> This section is managed by `generate-claude-profile` -- do not edit manually.
<!-- GSD:profile-end -->

<!-- rtk-instructions v2 -->
# RTK (Rust Token Killer) - Token-Optimized Commands

## Golden Rule

**Always prefix commands with `rtk`**. If RTK has a dedicated filter, it uses it. If not, it passes through unchanged. This means RTK is always safe to use.

**Important**: Even in command chains with `&&`, use `rtk`:
```bash
# ❌ Wrong
git add . && git commit -m "msg" && git push

# ✅ Correct
rtk git add . && rtk git commit -m "msg" && rtk git push
```

## RTK Commands by Workflow

### Build & Compile (80-90% savings)
```bash
rtk cargo build         # Cargo build output
rtk cargo check         # Cargo check output
rtk cargo clippy        # Clippy warnings grouped by file (80%)
rtk tsc                 # TypeScript errors grouped by file/code (83%)
rtk lint                # ESLint/Biome violations grouped (84%)
rtk prettier --check    # Files needing format only (70%)
rtk next build          # Next.js build with route metrics (87%)
```

### Test (90-99% savings)
```bash
rtk cargo test          # Cargo test failures only (90%)
rtk vitest run          # Vitest failures only (99.5%)
rtk playwright test     # Playwright failures only (94%)
rtk test <cmd>          # Generic test wrapper - failures only
```

### Git (59-80% savings)
```bash
rtk git status          # Compact status
rtk git log             # Compact log (works with all git flags)
rtk git diff            # Compact diff (80%)
rtk git show            # Compact show (80%)
rtk git add             # Ultra-compact confirmations (59%)
rtk git commit          # Ultra-compact confirmations (59%)
rtk git push            # Ultra-compact confirmations
rtk git pull            # Ultra-compact confirmations
rtk git branch          # Compact branch list
rtk git fetch           # Compact fetch
rtk git stash           # Compact stash
rtk git worktree        # Compact worktree
```

Note: Git passthrough works for ALL subcommands, even those not explicitly listed.

### GitHub (26-87% savings)
```bash
rtk gh pr view <num>    # Compact PR view (87%)
rtk gh pr checks        # Compact PR checks (79%)
rtk gh run list         # Compact workflow runs (82%)
rtk gh issue list       # Compact issue list (80%)
rtk gh api              # Compact API responses (26%)
```

### JavaScript/TypeScript Tooling (70-90% savings)
```bash
rtk pnpm list           # Compact dependency tree (70%)
rtk pnpm outdated       # Compact outdated packages (80%)
rtk pnpm install        # Compact install output (90%)
rtk npm run <script>    # Compact npm script output
rtk npx <cmd>           # Compact npx command output
rtk prisma              # Prisma without ASCII art (88%)
```

### Files & Search (60-75% savings)
```bash
rtk ls <path>           # Tree format, compact (65%)
rtk read <file>         # Code reading with filtering (60%)
rtk grep <pattern>      # Search grouped by file (75%)
rtk find <pattern>      # Find grouped by directory (70%)
```

### Analysis & Debug (70-90% savings)
```bash
rtk err <cmd>           # Filter errors only from any command
rtk log <file>          # Deduplicated logs with counts
rtk json <file>         # JSON structure without values
rtk deps                # Dependency overview
rtk env                 # Environment variables compact
rtk summary <cmd>       # Smart summary of command output
rtk diff                # Ultra-compact diffs
```

### Infrastructure (85% savings)
```bash
rtk docker ps           # Compact container list
rtk docker images       # Compact image list
rtk docker logs <c>     # Deduplicated logs
rtk kubectl get         # Compact resource list
rtk kubectl logs        # Deduplicated pod logs
```

### Network (65-70% savings)
```bash
rtk curl <url>          # Compact HTTP responses (70%)
rtk wget <url>          # Compact download output (65%)
```

### Meta Commands
```bash
rtk gain                # View token savings statistics
rtk gain --history      # View command history with savings
rtk discover            # Analyze Claude Code sessions for missed RTK usage
rtk proxy <cmd>         # Run command without filtering (for debugging)
rtk init                # Add RTK instructions to CLAUDE.md
rtk init --global       # Add RTK to ~/.claude/CLAUDE.md
```

## Token Savings Overview

| Category | Commands | Typical Savings |
|----------|----------|-----------------|
| Tests | vitest, playwright, cargo test | 90-99% |
| Build | next, tsc, lint, prettier | 70-87% |
| Git | status, log, diff, add, commit | 59-80% |
| GitHub | gh pr, gh run, gh issue | 26-87% |
| Package Managers | pnpm, npm, npx | 70-90% |
| Files | ls, read, grep, find | 60-75% |
| Infrastructure | docker, kubectl | 85% |
| Network | curl, wget | 65-70% |

Overall average: **60-90% token reduction** on common development operations.
<!-- /rtk-instructions -->