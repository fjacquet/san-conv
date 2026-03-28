# Phase 1: Foundation - Research

**Researched:** 2026-03-29
**Domain:** Go CLI project scaffolding — module init, cobra subcommands, IR struct definitions, golangci-lint v2, goreleaser v2
**Confidence:** HIGH

## Summary

Phase 1 delivers the compilable skeleton that unblocks all parallel parser and emitter work in Phases 2–7. The deliverable is not functional conversion logic — it is a project structure that compiles, stubs both subcommands, defines the full IR contract, and passes linting. Everything downstream depends on the IR structs being stable before those phases begin.

The stack is fully resolved in prior research (STACK.md): Go 1.26.1 is installed on this machine (newer than the 1.25 referenced in STACK.md — this is fine, no breaking changes relevant to this project). The `go mod init` command on this machine produces `go 1.26.1` in go.mod. The `tool` directive for dev tools (golangci-lint, goreleaser, cobra-cli) has been available since Go 1.24 and is fully supported. Both golangci-lint v2.11.4 and goreleaser v2.14.3 are installed at `/opt/homebrew/bin/`.

The key technical detail for this phase is the golangci-lint v2 configuration format — it requires `version: "2"` at the top of `.golangci.yml` (breaking change from v1). The default preset `linters.default: standard` enables exactly five linters: errcheck, govet, ineffassign, staticcheck, unused. That set is appropriate for a skeleton project. The goreleaser v2 minimal config requires `version: 2` (integer, not string) at the top.

**Primary recommendation:** Scaffold with `go mod init github.com/fjacquet/san-conv`, add cobra v1.10.2 as the only runtime dep, add golangci-lint/goreleaser/cobra-cli as `tool` directives, define the five IR structs in `internal/ir/zoningconfig.go`, stub both subcommands with `RunE` returning `fmt.Errorf("not implemented")`, write `.golangci.yml` with `version: "2"` and `linters.default: standard`, and write `.goreleaser.yml` with `version: 2` and the three target platforms.

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLI-07 | Single distributable Go binary with no runtime dependencies (go install or pre-built release) | goreleaser v2 cross-compilation config covers linux/amd64 + darwin/arm64 + windows/amd64; CGO_ENABLED=0 in builds section; go install works from go.mod module path |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.1 (installed) | Language runtime | Project constraint; single-binary, no runtime deps. `go mod init` produces `go 1.26.1` on this machine. |
| github.com/spf13/cobra | v1.10.2 | CLI subcommands, flags, --help generation | Industry standard (Kubernetes, Hugo, GitHub CLI); automatic --help; nested subcommands; POSIX flags. Verified December 2025. |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| github.com/stretchr/testify | v1.11.1 | Test assertions (`require` package) | All tests. Use `require` (not `assert`) — stops on first failure, correct for parser tests. Verified August 2025. |
| github.com/spf13/pflag | v1.0.6 | POSIX flag parsing | Transitive via cobra — do NOT use directly. Access flags via `cmd.Flags()` and `cmd.PersistentFlags()`. |

### Development Tools (go.mod `tool` directive)

| Tool | Version | Purpose | Notes |
|------|---------|---------|-------|
| golangci-lint | v2.11.4 | Static analysis, linting | Installed at `/opt/homebrew/bin/golangci-lint`. Run via `go tool golangci-lint run ./...`. |
| goreleaser | v2.14.3 | Cross-platform binary release | Installed at `/opt/homebrew/bin/goreleaser`. Run via `go tool goreleaser release --snapshot --clean`. |
| cobra-cli | v1.3.0 | Cobra command scaffolding | Run via `go tool cobra-cli add <command>` to generate consistent command files. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| cobra | kong, urfave/cli v3 | cobra has broader docs/ecosystem for 2-3 subcommand tools; switch only if subcommands exceed ~10 |
| stdlib log/slog | zerolog, logrus | zerolog is faster but irrelevant for a CLI tool; logrus is maintenance-mode; slog has zero external dep |

**Installation:**

```bash
go mod init github.com/fjacquet/san-conv
go get github.com/spf13/cobra@v1.10.2
go get github.com/stretchr/testify@v1.11.1
go get -tool github.com/spf13/cobra-cli@latest
go get -tool github.com/golangci/golangci-lint/cmd/golangci-lint@v2.11.4
go get -tool github.com/goreleaser/goreleaser@v2.14.3
```

**Version verification (run before coding):**

```bash
go version                                     # expect go1.26.1
/opt/homebrew/bin/golangci-lint version        # expect 2.11.4
/opt/homebrew/bin/goreleaser --version         # expect GitVersion: 2.14.3
go list -m -json github.com/spf13/cobra@latest # confirm v1.10.2
go list -m -json github.com/stretchr/testify@latest # confirm v1.11.1
```

## Architecture Patterns

### Recommended Project Structure

```
san-conv/
├── go.mod                          # module github.com/fjacquet/san-conv; go 1.26.1
├── go.sum
├── .golangci.yml                   # golangci-lint v2 config
├── .goreleaser.yml                 # goreleaser v2 config
├── main.go                         # package main; calls cmd.Execute()
├── cmd/
│   ├── root.go                     # rootCmd, Execute(), persistent flags
│   ├── mds2brocade.go              # mds2brocadeCmd stub
│   └── brocade2mds.go              # brocade2mdsCmd stub
├── internal/
│   ├── ir/
│   │   └── zoningconfig.go         # ZoningConfig, Alias, Zone, ZoneMember, ZoneConfig structs
│   ├── parser/
│   │   ├── mds/                    # Phase 2: MDS parser (empty, create package stub)
│   │   └── brocade/                # Phase 3: Brocade parser (empty, create package stub)
│   ├── validator/                  # Phase 4: Validator (empty, create package stub)
│   ├── emitter/
│   │   ├── brocade/                # Phase 5: Brocade emitter (empty, create package stub)
│   │   └── mds/                    # Phase 6: MDS emitter (empty, create package stub)
│   └── converter/                  # Phase 7: Pipeline wiring (empty, create package stub)
└── testdata/
    ├── mds/                        # Phase 2: MDS fixture files (empty dir, .gitkeep)
    └── brocade/                    # Phase 3: Brocade fixture files (empty dir, .gitkeep)
```

**Why this layout:** `internal/` enforces that no external code can import these packages (Go toolchain enforced). `cmd/` is the cobra layer — it wires flags and calls internal packages but contains no conversion logic. Each `internal/` subdirectory maps to exactly one phase, so directory creation in Phase 1 unblocks all parallel phases.

**Note on empty package stubs:** Each empty sub-package in `internal/` needs at least one `.go` file declaring `package <name>` so the directory is recognized by the Go toolchain. A single `doc.go` file with `// Package name provides ...` and `package name` is sufficient.

### Pattern 1: Cobra Root + Subcommand Structure

**What:** Root command delegates to subcommands; subcommands use `RunE` for error handling.
**When to use:** Always — `RunE` returns proper exit codes without manual `os.Exit`.

```go
// main.go
package main

import "github.com/fjacquet/san-conv/cmd"

func main() {
    cmd.Execute()
}
```

```go
// cmd/root.go
package cmd

import (
    "os"
    "github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
    Use:   "san-conv",
    Short: "Convert SAN zoning configurations between Cisco MDS and Brocade FOS formats",
    Long: `san-conv converts SAN fabric zoning configurations between
Cisco MDS NX-OS and Brocade FOS formats.

Primary use case: mds2brocade (MDS running-config → FOS CLI commands)
Reverse direction: brocade2mds (FOS cfgshow/script → NX-OS CLI commands)`,
}

func Execute() {
    if err := rootCmd.Execute(); err != nil {
        os.Exit(1)
    }
}

func init() {
    rootCmd.AddCommand(mds2brocadeCmd)
    rootCmd.AddCommand(brocade2mdsCmd)
}
```

```go
// cmd/mds2brocade.go
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var mds2brocadeCmd = &cobra.Command{
    Use:   "mds2brocade [input-file]",
    Short: "Convert Cisco MDS NX-OS running-config to Brocade FOS CLI commands",
    Long: `mds2brocade parses a Cisco MDS NX-OS running-config file and produces
ready-to-apply Brocade FOS CLI commands (alicreate, zonecreate, cfgcreate).

The output includes a defzone --noaccess preamble and cfgsave postamble.
cfgenable is present as a commented-out line requiring human confirmation.`,
    Args: cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return fmt.Errorf("mds2brocade: not yet implemented")
    },
}

func init() {
    // Flags will be added in Phase 7 (CLI Wiring)
    // Stub them here as empty declarations so --help shows them
    mds2brocadeCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
    mds2brocadeCmd.Flags().String("script", "", "also write executable shell script to file")
    mds2brocadeCmd.Flags().String("fos-version", "pre-8.1", "target FOS naming rules (pre-8.1 or 8.1+)")
}
```

```go
// cmd/brocade2mds.go  — symmetric stub
package cmd

import (
    "fmt"
    "github.com/spf13/cobra"
)

var brocade2mdsCmd = &cobra.Command{
    Use:   "brocade2mds [input-file]",
    Short: "Convert Brocade FOS cfgshow or CLI script to Cisco MDS NX-OS commands",
    Long: `brocade2mds parses a Brocade FOS cfgshow output or CLI script and produces
NX-OS CLI commands (device-alias database, zone, zoneset, zoneset activate).`,
    Args: cobra.MaximumNArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        return fmt.Errorf("brocade2mds: not yet implemented")
    },
}

func init() {
    brocade2mdsCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
}
```

### Pattern 2: IR Struct Definitions

**What:** The Intermediate Representation (IR) structs are the shared data contract for the entire pipeline.
**When to use:** Define once in Phase 1; never change without updating all phases.
**Critical constraint:** IR structs must be finalized before any parser or emitter work begins.

```go
// internal/ir/zoningconfig.go
package ir

// ZoningConfig is the canonical format-neutral representation of a SAN zoning
// configuration. All parsers produce a *ZoningConfig; all emitters consume one.
// The IR is intentionally simple: no methods, no logic, no external dependencies.
type ZoningConfig struct {
    // Aliases maps alias name → Alias (both device-alias and fcalias from MDS;
    // alishow entries from Brocade). Key is the original source name (pre-sanitization).
    Aliases map[string]*Alias

    // Zones maps zone name → Zone, scoped per VSAN.
    // For Brocade (single-fabric, no VSANs), all zones use VSAN 0 as a sentinel.
    Zones map[string]*Zone

    // ZoneConfigs maps cfgname → ZoneConfig (Brocade cfg; MDS zoneset).
    ZoneConfigs map[string]*ZoneConfig

    // SourceFormat identifies the parser that produced this IR.
    // Values: "mds-nxos" | "brocade-fos"
    SourceFormat string

    // Warnings accumulates non-fatal issues discovered during parsing.
    // Parsers append to this slice; the CLI layer prints it to stderr.
    Warnings []string
}

// Alias represents a single named WWN mapping.
// In MDS: device-alias or fcalias entry.
// In Brocade: alicreate entry.
type Alias struct {
    Name string // Original source name (pre-sanitization)
    PWWN string // Port WWN in normalized lowercase colon-hex: "10:00:00:00:c9:12:34:56"
    VSAN int    // VSAN scope (MDS fcalias only); 0 means fabric-wide (device-alias or Brocade)
}

// Zone represents a single zone definition.
// In MDS: zone name X vsan Y block.
// In Brocade: zonecreate zone entry.
type Zone struct {
    Name    string        // Original zone name (pre-sanitization)
    VSAN    int           // VSAN scope; 0 for Brocade (no VSAN concept)
    Members []*ZoneMember // Ordered list of zone members
}

// ZoneMember represents a single member within a zone.
// Members can be raw pWWNs, alias references, or unsupported types.
type ZoneMember struct {
    // Type indicates the member variant:
    //   "pwwn"   — raw pWWN (always resolvable to FOS)
    //   "alias"  — reference to an Alias by name (device-alias or fcalias)
    //   "unsupported" — interface, fcid, ip-address, etc. (skipped with warning)
    Type string

    // Value holds the member value appropriate to Type:
    //   "pwwn":        the pWWN string
    //   "alias":       the alias name
    //   "unsupported": original member string (for warning message)
    Value string
}

// ZoneConfig represents a zone set / configuration.
// In MDS: zoneset name X vsan Y.
// In Brocade: cfg (cfgcreate).
type ZoneConfig struct {
    Name      string   // Original cfg/zoneset name (pre-sanitization)
    VSAN      int      // VSAN scope (MDS); 0 for Brocade
    ZoneNames []string // Ordered list of zone names in this config/zoneset
}
```

**IR design rationale:**
- Zero methods, zero logic: IR is pure data. No `Validate()`, no `Sanitize()` methods — those belong in `internal/validator/`.
- Map keys use original names: Parsers store pre-sanitization names; the validator/sanitizer produces a new sanitized map. This separation prevents lossy data during validation.
- `Warnings []string` on `ZoningConfig`: Parsers accumulate non-fatal warnings inline. The CLI layer drains this slice to stderr. This keeps parsers standalone and testable.
- `VSAN int` with 0 as sentinel: Brocade has no VSAN concept; using 0 unifies the struct without adding a separate Brocade-specific type.

### Pattern 3: Empty Package Stub

**What:** Every `internal/` subdirectory needs a compilable Go file to exist in the module graph.
**When to use:** For all six internal sub-packages not implemented in Phase 1.

```go
// internal/parser/mds/doc.go
// Package mds implements the Cisco MDS NX-OS running-config parser.
// It produces a *ir.ZoningConfig from a full NX-OS running-config file.
// Implemented in Phase 2.
package mds
```

Create the same pattern for: `internal/parser/brocade/`, `internal/validator/`, `internal/emitter/brocade/`, `internal/emitter/mds/`, `internal/converter/`.

### Anti-Patterns to Avoid

- **`Run` instead of `RunE`:** `Run` ignores errors; `RunE` returns them and cobra exits non-zero automatically. Always use `RunE`.
- **Direct `os.Exit()` in commands:** Cobra handles exit codes from `RunE` errors. Calling `os.Exit()` in command functions bypasses deferred cleanup.
- **Logic in `internal/ir/`:** The IR package must have zero logic (no methods that modify state, no validation). Any code that goes beyond struct definitions belongs in `internal/validator/` or parsers.
- **`html/template` for output:** It HTML-escapes `"`, `<`, `>` — this corrupts FOS command strings. Always use `text/template`.
- **`import cycles`:** cobra commands in `cmd/` may import `internal/ir` but `internal/ir` must import nothing from this module. The IR package has zero internal imports by design.
- **Adding all linters in golangci-lint:** `linters.default: standard` (errcheck, govet, ineffassign, staticcheck, unused) is appropriate for a skeleton. Adding all linters on an empty project produces false positives from stubbed code and makes CI noisy before any real code exists.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| CLI subcommands and --help | Custom flag parsing | cobra v1.10.2 | Cobra handles --help generation, usage strings, arg validation, shell completion, POSIX flags, and subcommand routing. Reimplementing even basic --help correctly takes 200+ lines. |
| Cross-platform binary releases | Manual `go build` scripts | goreleaser v2.14.3 | Goreleaser handles platform matrix, archive formats (tar.gz/zip), checksum files, GitHub release drafts, and optional Homebrew tap. Manual scripts diverge from goreleaser's conventions quickly. |
| Static analysis / linting | Custom lint rules | golangci-lint v2.11.4 | Golangci-lint aggregates 90+ linters with deduplication and performance tuning. Writing custom lint rules for standard Go issues (unchecked errors, shadowed variables) is not justified. |
| Dev tool version pinning | tools.go blank import trick | go.mod `tool` directive (Go 1.24+) | The `tool` directive is the official supported mechanism since Go 1.24. The `tools.go` workaround is obsolete and creates phantom import cycles. |

**Key insight:** This phase is infrastructure — use the established tools exactly as they are designed. Custom scripts or workarounds here create maintenance debt that compounds across all 7 phases.

## Common Pitfalls

### Pitfall 1: golangci-lint v1 vs v2 Config Format
**What goes wrong:** `.golangci.yml` without `version: "2"` at the top is treated as v1 format. In v2, the v1 format produces deprecation warnings or errors. The `enable-all:` and `disable-all:` keys from v1 do not exist in v2.
**Why it happens:** Documentation for golangci-lint v1 is still widely indexed; many examples online predate v2.
**How to avoid:** First line of `.golangci.yml` MUST be `version: "2"` (quoted string). Use `linters.default: standard` instead of any v1 enable/disable list.
**Warning signs:** `golangci-lint run` prints "configuration file contains unknown key" or migration prompts.

### Pitfall 2: goreleaser v2 vs v1 Config Format
**What goes wrong:** `.goreleaser.yml` without `version: 2` at the top is treated as v1 schema. v2 uses integer `2`, not string `"2"`. Missing this causes schema validation errors.
**Why it happens:** v1 config had no version key; adding it with wrong type (string vs integer) silently falls back to wrong schema.
**How to avoid:** First line of `.goreleaser.yml` MUST be `version: 2` (unquoted integer). Run `goreleaser check` to validate config.
**Warning signs:** goreleaser outputs "configuration file is a v1 config" warning.

### Pitfall 3: go.mod Module Path Mismatch
**What goes wrong:** If the module path in `go.mod` does not match the GitHub repository path, `go install github.com/fjacquet/san-conv@latest` fails. All import paths in `cmd/` and `internal/` reference the module path — changing it later requires updating all import statements.
**Why it happens:** Developers use `go mod init san-conv` (just the name) during local development but the correct path includes the full GitHub org prefix.
**How to avoid:** Use `go mod init github.com/fjacquet/san-conv` on the first `go mod init` run. Never change this after code exists.
**Warning signs:** `go install` fails with "no Go files in..." or "cannot find module providing...".

### Pitfall 4: IR Import Cycles
**What goes wrong:** If any `internal/` package (parser, validator, emitter) imports another `internal/` package other than `internal/ir`, an import cycle is likely. For example: `internal/parser/mds` importing `internal/validator` creates a cycle if validator ever imports a parser type.
**Why it happens:** The compiler pipeline pattern requires a strict DAG. Accidentally adding cross-package imports at the stub stage is easier to prevent than to untangle later.
**How to avoid:** The only cross-package import allowed in this phase is: `cmd/` → `internal/ir`. All other `internal/` packages in Phase 1 are empty stubs with no imports. Enforce with `golangci-lint`'s import cycle detection.
**Warning signs:** `go build` reports "import cycle not allowed".

### Pitfall 5: Stub `RunE` Not Returning Error
**What goes wrong:** A stub subcommand that returns `nil` from `RunE` (instead of `fmt.Errorf("not implemented")`) passes silently in integration tests without actually doing anything. This masks missing implementation in Phase 7.
**Why it happens:** Copy-paste from tutorials that show `return nil` in `RunE`.
**How to avoid:** Every stub `RunE` in Phase 1 MUST return `fmt.Errorf("%s: not yet implemented", cmd.Use)`. This causes the command to exit non-zero when called, making it obvious the stub is active.
**Warning signs:** `san-conv mds2brocade somefile.txt` exits 0 with no output.

### Pitfall 6: go 1.26 `go.mod` `go` Directive vs Minimum Version
**What goes wrong:** `go mod init` on this machine produces `go 1.26.1` in `go.mod`. This sets the minimum Go version for anyone building the tool. If the ops team's Go installation is older (e.g., 1.21.x), they cannot build from source.
**Why it happens:** `go mod init` automatically uses the installed Go version.
**How to avoid:** Decide the minimum Go version the project targets. The `tool` directive requires Go 1.24+. If Go 1.24 is acceptable as minimum, change `go 1.26.1` to `go 1.24.0` in `go.mod` after `go mod init`. For a new project where ops team downloads pre-built binaries (via goreleaser), this is a non-issue — source builds are only for the developer.
**Warning signs:** `go build` on an older Go installation fails with "note: module requires Go 1.26.1".

## Code Examples

### Complete `.golangci.yml` for Phase 1 Skeleton

```yaml
# Source: https://golangci-lint.run/docs/configuration/file/
# golangci-lint v2 format — version key is REQUIRED
version: "2"

linters:
  # standard preset enables: errcheck, govet, ineffassign, staticcheck, unused
  default: standard
  enable:
    - gofmt       # enforce gofmt formatting
    - misspell    # catch typos in comments and strings

run:
  timeout: 5m
  tests: true

issues:
  max-issues-per-linter: 50
  max-same-issues: 3
```

**Note:** `gofmt` and `misspell` are in the disabled-by-default list but are worth enabling immediately. All other additional linters are best added in later phases when real code exists to lint.

### Complete `.goreleaser.yml` for Phase 1

```yaml
# Source: https://goreleaser.com/customization/builds/builders/go/
# goreleaser v2 format — version key is REQUIRED (integer, not string)
version: 2

project_name: san-conv

before:
  hooks:
    - go mod download
    - go mod tidy

builds:
  - id: san-conv
    main: .
    binary: san-conv
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    # Exclude windows/arm64 (not a common ops team platform)
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w
      - -X main.version={{.Version}}

archives:
  - id: san-conv
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"

snapshot:
  name_template: "{{ .Tag }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
```

**Testing the config:** `go tool goreleaser check` validates the YAML schema. `go tool goreleaser release --snapshot --clean` builds all platforms locally without publishing.

### Complete `internal/ir/zoningconfig.go`

(See Architecture Patterns section above — the full struct definitions are provided there.)

### `go.mod` Structure After Phase 1 Init

```
module github.com/fjacquet/san-conv

go 1.26.1

require (
    github.com/spf13/cobra v1.10.2
)

require (
    github.com/spf13/pflag v1.0.6 // indirect
)

require (
    github.com/stretchr/testify v1.11.1
)

tool (
    github.com/goreleaser/goreleaser
    github.com/golangci/golangci-lint/cmd/golangci-lint
    github.com/spf13/cobra-cli
)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `tools.go` blank imports for dev tools | `tool` directive in `go.mod` | Go 1.24 (Feb 2024) | Eliminates phantom blank import file; tools tracked in go.mod like any dependency |
| golangci-lint v1 `enable-all:` / `disable-all:` | `linters.default: standard/all/none/fast` | golangci-lint v2.0.0 (March 2025) | Breaking config change — v1 YAML fails in v2 without `golangci-lint migrate` |
| goreleaser v1 config (no version key) | `version: 2` required at top of `.goreleaser.yml` | goreleaser v2.0.0 | Breaking change — v2 requires explicit version declaration |
| `os.Exit(1)` in cobra commands | `RunE` returning error | cobra best practice, always | Cobra handles exit codes from RunE; os.Exit skips deferred cleanup |

**Deprecated/outdated:**
- `tools.go` pattern: Replaced by `go.mod tool` directive in Go 1.24+. Still works but is no longer idiomatic.
- golangci-lint v1 YAML format: Use `golangci-lint migrate` to convert if upgrading an existing project.
- `cobra.CheckErr(err)` in `main()`: Replaced by `os.Exit(1)` after `rootCmd.Execute()` returns an error, per current cobra docs.

## Open Questions

1. **Go module path: `github.com/fjacquet/san-conv` vs private/internal path**
   - What we know: Research used this path; it matches the likely GitHub location.
   - What's unclear: Whether the repository is public on GitHub at this path or will be hosted elsewhere.
   - Recommendation: Use `github.com/fjacquet/san-conv` as documented in the research. Change it before the first commit if the repo path differs — changing it after code exists requires updating all internal import paths.

2. **Minimum Go version for `go.mod` directive**
   - What we know: System has Go 1.26.1; `go mod init` will write `go 1.26.1`. The `tool` directive requires Go 1.24+.
   - What's unclear: Whether the ops team needs to build from source (which would require Go 1.26+), or whether they will only use pre-built goreleaser binaries.
   - Recommendation: For an ops tool distributed via pre-built binaries, keep `go 1.26.1` as the `go.mod` directive — only the developer needs Go 1.26+. If source builds for ops team are a requirement, lower it to `go 1.24.0` (minimum for `tool` directive).

3. **`internal/converter/` vs direct wiring in `cmd/`**
   - What we know: The SUMMARY.md mentions a converter package; the STACK.md and ARCHITECTURE.md notes describe the CLI layer as the pipeline orchestrator.
   - What's unclear: Whether Phase 7's CLI wiring logic belongs in `cmd/` directly or needs a separate `internal/converter/` intermediary.
   - Recommendation: Create the `internal/converter/` stub in Phase 1 (single `doc.go` file) to reserve the namespace. The actual placement decision can be deferred to Phase 7 without cost.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All tasks | Yes | 1.26.1 darwin/arm64 | — |
| golangci-lint | Lint pass (success criterion #5) | Yes | v2.11.4 | — |
| goreleaser | Goreleaser config validation | Yes | v2.14.3 | — |
| git | Module init, tracking | Yes | (system git) | — |

**No missing dependencies.** All tools required for Phase 1 are installed and available at known paths.

## Sources

### Primary (HIGH confidence)
- Go official module layout docs: https://go.dev/doc/modules/layout — cmd/ + internal/ layout for CLI tools
- Go 1.24 release notes: https://go.dev/doc/go1.24 — `tool` directive in go.mod
- Go 1.26 release notes: https://go.dev/doc/go1.26 — no breaking changes for cobra/stdlib CLI projects
- cobra official docs: https://cobra.dev/docs/how-to-guides/working-with-commands/ — RunE, stub pattern, init() registration
- golangci-lint config docs: https://golangci-lint.run/docs/configuration/file/ — version: "2" requirement, linters.default
- goreleaser builds docs: https://goreleaser.com/customization/builds/builders/go/ — goos/goarch config, version: 2

### Secondary (MEDIUM confidence)
- ldez.github.io golangci-lint v2 migration guide — breaking changes from v1 to v2 confirmed
- WebSearch: goreleaser v2 minimal config examples — multiple sources agree on version: 2 + CGO_ENABLED=0 pattern
- WebSearch: cobra project layout best practices — multiple sources agree on cmd/root.go + cmd/subcommand.go pattern

### Verified from environment
- `go version` → go1.26.1 darwin/arm64 (HIGH — direct check)
- `/opt/homebrew/bin/golangci-lint version` → v2.11.4 (HIGH — direct check)
- `/opt/homebrew/bin/goreleaser --version` → GitVersion: 2.14.3 (HIGH — direct check)
- `go list -m -json github.com/spf13/cobra@latest` → v1.10.2, Dec 2025 (HIGH — module proxy)
- `go list -m -json github.com/stretchr/testify@latest` → v1.11.1, Aug 2025 (HIGH — module proxy)
- `golangci-lint help linters` → default linters: errcheck, govet, ineffassign, staticcheck, unused (HIGH — direct check)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all versions verified via module proxy and direct tool invocation on this machine
- Architecture: HIGH — Go module layout from official docs; IR struct design from prior ARCHITECTURE.md research; cobra patterns from official cobra.dev docs
- Pitfalls: HIGH — golangci-lint v2 format change verified via direct linter invocation; goreleaser v2 format from official docs; import cycle prevention is Go toolchain-enforced

**Research date:** 2026-03-29
**Valid until:** 2026-06-29 (stable ecosystem — cobra, goreleaser, golangci-lint have slow release cadences)
