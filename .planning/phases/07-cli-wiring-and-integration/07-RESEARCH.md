# Phase 7: CLI Wiring and Integration - Research

**Researched:** 2026-03-29
**Domain:** Go CLI integration — Cobra command wiring, io.Writer pipeline, exit codes, stderr summary
**Confidence:** HIGH

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CLI-01 | Tool accepts input config file path as positional argument | Cobra `Args: cobra.ExactArgs(1)` pattern; read `args[0]` in RunE |
| CLI-02 | Tool provides `--direction` flag (mds2brocade default, brocade2mds) | Flag already partially stubbed; needs to be wired to dispatch logic |
| CLI-03 | Tool provides `--output` flag to write primary output to file instead of stdout | `os.Create` + close; `io.Writer` already matches emitter signature |
| CLI-04 | Tool provides `--script` flag to write shell script file alongside primary output | Second `io.Writer` path through `brocade.Emit(cfg, scriptW, true)` |
| CLI-05 | Tool provides `--fos-version` flag (pre-8.1 / 8.1+) defaulting to pre-8.1 | Passed to `validator.Sanitize(cfg, fosVersion)` |
| CLI-06 | Tool exits code 0 on success (warnings allowed), non-zero on fatal IO errors | `RunE` returns error → cobra calls `os.Exit(1)`; warnings only → return nil |
| OUT-04 | Tool prints conversion summary to stderr: objects converted, skipped, warnings | `fmt.Fprintf(os.Stderr, ...)` after emit; count from cfg.Warnings + IR maps |

</phase_requirements>

---

## Summary

Phase 7 is the integration phase: the pipeline already exists (parsers, validator, emitters are all complete and tested), so this phase is purely about wiring. The `internal/converter` package has a `doc.go` placeholder explicitly noting it is "implemented in Phase 7." That package is the right place to put a `Run(opts Options, stdout io.Writer, stderr io.Writer) error` function that orchestrates the full pipeline.

The two cobra command files (`cmd/mds2brocade.go` and `cmd/brocade2mds.go`) already have flags stubbed but `RunE` returns `fmt.Errorf("not yet implemented")`. The core work of Phase 7 is replacing those stubs with real dispatch to `internal/converter`. The root command currently uses two subcommands; the requirements now want a flat invocation `san-conv myconfig.txt` (defaulting to mds2brocade direction) plus an optional `--direction` flag. This means the design question is: flat root command with a `--direction` flag, or keep the subcommand structure. The requirements description says `san-conv myconfig.txt` (no subcommand), so the Makefile already anticipates this: `run-mds` invokes `./$(BINARY) mds2brocade $(INPUT)`. However SUCCESS CRITERIA #1 explicitly says `san-conv myconfig.txt` runs mds2brocade — this strongly implies the root command itself should accept the file argument with `--direction` flag, OR the `mds2brocade` subcommand remains but the root command also handles the case with a default.

The safest interpretation that matches all six success criteria: wire `mds2brocadeCmd` fully (covers SC#1 via `san-conv mds2brocade myconfig.txt` or change root to dispatch). Both paths are valid; the planner must pick one and make it consistent with the existing Makefile `run-mds` target.

**Primary recommendation:** Implement `internal/converter` as a direction-agnostic orchestrator. Wire the existing `mds2brocadeCmd` and `brocade2mdsCmd` cobra commands as the primary interface. Optionally add a root-level default that delegates to `mds2brocadeCmd` for a bare `san-conv file.txt` invocation. This is less disruptive than deleting the subcommand structure.

---

## Standard Stack

### Core (already in go.mod — no new installs needed)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/spf13/cobra` | v1.10.2 | CLI subcommands, flags, RunE | Already in use throughout project |
| `github.com/stretchr/testify/require` | v1.11.1 | Test assertions | Already used in all test files |
| stdlib `os` | Go 1.26.1 | File open/create, os.Stderr, os.Exit | No external dep needed |
| stdlib `fmt` | Go 1.26.1 | Fprintf to stderr for summary | No external dep needed |
| stdlib `io` | Go 1.26.1 | io.Writer interface threading | Already used in both emitters |
| stdlib `log/slog` | Go 1.21+ | Structured warnings if needed | Already in project stack |

**Installation:** No new packages required. All dependencies are already in `go.mod`.

### No New Dependencies

This phase introduces zero new packages. Every library needed is already present.

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
├── converter/
│   ├── doc.go            # Exists — "implemented in Phase 7"
│   └── converter.go      # NEW: Run() function orchestrating pipeline
cmd/
├── root.go               # Possibly add PersistentPreRunE or Args handling
├── mds2brocade.go        # Wire RunE to converter.Run()
└── brocade2mds.go        # Wire RunE to converter.Run()
```

### Pattern 1: Converter Package as Pipeline Orchestrator

**What:** `internal/converter/converter.go` defines an `Options` struct and a `Run` function that sequences: open file → parse → sanitize → emit → print summary.
**When to use:** Any time the CLI layer needs to invoke the pipeline. The converter is the single seam between cobra commands and internal packages.

```go
// internal/converter/converter.go
package converter

import (
    "fmt"
    "io"
    "os"

    brocadeemitter "github.com/fjacquet/san-conv/internal/emitter/brocade"
    mdsemitter     "github.com/fjacquet/san-conv/internal/emitter/mds"
    brocadeparser  "github.com/fjacquet/san-conv/internal/parser/brocade"
    mdsparser      "github.com/fjacquet/san-conv/internal/parser/mds"
    "github.com/fjacquet/san-conv/internal/validator"
)

// Options holds all user-supplied parameters for a conversion run.
type Options struct {
    InputFile  string    // Path to input config file (CLI-01)
    Direction  string    // "mds2brocade" or "brocade2mds" (CLI-02)
    OutputFile string    // "" means stdout (CLI-03)
    ScriptFile string    // "" means no script (CLI-04)
    FOSVersion string    // "pre-8.1" or "8.1+" (CLI-05)
}

// Run executes the full conversion pipeline and returns an error only for fatal
// IO or parse failures. Warnings are printed to stderr. Exit code is controlled
// by whether Run returns nil or an error (CLI-06).
func Run(opts Options, stdout io.Writer, stderr io.Writer) error {
    // 1. Open input file (fatal on error)
    f, err := os.Open(opts.InputFile)
    if err != nil {
        return fmt.Errorf("opening input: %w", err)
    }
    defer f.Close()

    // 2. Parse
    // 3. Sanitize (mds2brocade only)
    // 4. Emit to primary output writer
    // 5. Emit script (mds2brocade only, if ScriptFile != "")
    // 6. Print summary to stderr (OUT-04)
    _ = stdout
    _ = stderr
    _ = brocadeemitter.Emit
    _ = mdsemitter.Emit
    _ = brocadeparser.Parse
    _ = mdsparser.Parse
    _ = validator.Sanitize
    return nil
}
```

**Key principle:** `Run` never calls `os.Exit` directly — it returns an error. The cobra `RunE` handler returns that error, and `rootCmd.Execute()` in `cmd/root.go` calls `os.Exit(1)` when Execute returns a non-nil error. This is already wired correctly: `root.go` line 22 does `os.Exit(1)` on error.

### Pattern 2: Output Writer Resolution

**What:** Convert `--output` and `--script` flag values to `io.Writer` values before calling `converter.Run`.
**When to use:** In `RunE` of each cobra command, before handing off to the converter.

```go
// cmd/mds2brocade.go — RunE body
func(cmd *cobra.Command, args []string) error {
    outputFile, _ := cmd.Flags().GetString("output")
    scriptFile,  _ := cmd.Flags().GetString("script")
    fosVersion,  _ := cmd.Flags().GetString("fos-version")

    return converter.Run(converter.Options{
        InputFile:  args[0],
        Direction:  "mds2brocade",
        OutputFile: outputFile,
        ScriptFile: scriptFile,
        FOSVersion: fosVersion,
    }, os.Stdout, os.Stderr)
}
```

The converter internally resolves `OutputFile == ""` to `stdout` and opens the file otherwise. **Close the file inside the converter, not the CLI layer**, to keep resource management co-located with acquisition.

### Pattern 3: Summary Output (OUT-04)

**What:** After emit, print a one-line summary to stderr with object counts.
**When to use:** At the end of every successful `Run` call, before returning nil.

```go
// Summary counts derived from the IR after sanitization
aliases := len(cfg.Aliases)
zones   := len(cfg.Zones)
cfgs    := len(cfg.ZoneConfigs)
warns   := len(cfg.Warnings)

// Print warnings first so the summary line is always the last stderr line
for _, w := range cfg.Warnings {
    fmt.Fprintf(stderr, "WARN: %s\n", w)
}
fmt.Fprintf(stderr, "Summary: %d aliases, %d zones, %d configs converted; %d warnings\n",
    aliases, zones, cfgs, warns)
```

**Why stderr:** stdout is reserved for machine-consumable FOS/NX-OS commands. Mixing summary prose into stdout would break piped workflows (ops team might pipe output directly to `ssh switch`).

### Pattern 4: Cobra Exit Code Contract (CLI-06)

**What:** `RunE` returns `error` — cobra propagates this to `Execute()`, which returns the error, and `root.go`'s `os.Exit(1)` fires.
**Critical detail:** Cobra by default prints the error AND the usage text when `RunE` returns an error. For a CLI tool, printing usage on every runtime error (e.g., "file not found") is noisy. Suppress usage printing on runtime errors:

```go
// In RunE, before returning a runtime error:
cmd.SilenceUsage = true  // Suppress usage on runtime errors (not flag parse errors)
```

Or set it once on the root command:
```go
// cmd/root.go init()
rootCmd.SilenceErrors = false  // Let cobra print the error message
rootCmd.SilenceUsage  = true   // But don't print full usage for every runtime error
```

**Existing pattern in codebase:** `root.go` currently uses `Run` pattern (calls `os.Exit(1)` directly). The individual command files use `RunE`. This is the correct pattern — keep it.

### Pattern 5: Positional Argument Design (CLI-01)

**Current state:** Both commands have `Args: cobra.MaximumNArgs(1)` — the input file is optional.

**Required state per success criteria:** `san-conv myconfig.txt` (no subcommand prefix in SC#1). Two valid options:

**Option A — Keep subcommands, update root to forward bare args:**
Add a `ValidArgsFunction` or `Run` to `rootCmd` that detects a file argument and defaults to `mds2brocade`. This is complex and fragile.

**Option B — Change `Args` to `cobra.ExactArgs(1)` on both subcommands, accept that users type `san-conv mds2brocade myconfig.txt`:**
The Makefile `run-mds` target already uses this form. SC#1 may be describing the _logical_ intent (input file is the primary arg), not necessarily the literal invocation without a subcommand. The planner should confirm this interpretation.

**Option C — Collapse to root command with `--direction` flag:**
Remove subcommands. Single `rootCmd` accepts `Args: cobra.ExactArgs(1)` and `--direction` flag. This matches SC#1 most literally but requires deleting the subcommand infrastructure.

**Recommendation:** Option B is lowest risk. Change `Args` from `MaximumNArgs(1)` to `ExactArgs(1)` on both subcommands. Fail fast with a clear error if no file is provided. SC#1's `san-conv myconfig.txt` can be interpreted as `san-conv mds2brocade myconfig.txt` with mds2brocade as the default/primary command.

### Anti-Patterns to Avoid

- **Direct `os.Exit` in converter:** Makes the converter untestable. The converter returns errors; only the CLI entry point calls `os.Exit`.
- **Writing to `os.Stdout` directly in converter:** Prevents testing without stdout capture. Always thread `io.Writer` parameters.
- **Writing warnings to stdout:** Breaks pipe workflows. Warnings and summary go to `stderr`; FOS/NX-OS commands go to `stdout`.
- **Opening files in RunE and passing file handles to converter:** Leaks resources if converter errors mid-run. Converter should own file lifecycle.
- **Not setting `cmd.SilenceUsage = true`:** Users see the full `--help` text when a file doesn't exist, which is confusing.
- **Calling `validator.Sanitize` for brocade2mds direction:** The sanitizer applies FOS naming rules. For brocade2mds (producing NX-OS output), FOS rules should NOT be applied to names that will become NX-OS names. The sanitizer is mds2brocade-only.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Exit code on error | Manual `os.Exit` calls in converter | Return error from RunE | Cobra's Execute() already wires os.Exit(1) in root.go |
| Flag default values | Conditional logic in RunE | Cobra flags with `.String("flag", "default", "desc")` | Already stubbed in both command files |
| File writing with fallback to stdout | `if outputFile == "" { use stdout } else { open file }` | Exactly this — it's 4 lines, don't abstract further | No library needed for this simple pattern |
| Cross-platform binary | Manual build matrix | goreleaser — already configured in `.goreleaser.yml` | goreleaser handles linux/darwin/windows, checksums, GitHub release assets |

**Key insight:** Phase 7 is plumbing, not invention. Every complex piece (parsing, sanitization, emission) is already implemented and tested. The only new code is the converter orchestrator and flag wiring.

---

## Common Pitfalls

### Pitfall 1: Sanitizer applied in wrong direction

**What goes wrong:** `validator.Sanitize(cfg, fosVersion)` is called for the brocade2mds direction, replacing characters in Brocade names before they become NX-OS device-alias names. NX-OS accepts a wider character set than FOS (including hyphens), so the sanitizer would corrupt valid names unnecessarily.
**Why it happens:** It's easy to call Sanitize unconditionally in the converter without thinking about direction semantics.
**How to avoid:** Only call `Sanitize` when `opts.Direction == "mds2brocade"`. For brocade2mds, emit directly from parsed IR.
**Warning signs:** Test: a Brocade alias named `host-01` becomes `host_01` in NX-OS output — that's wrong.

### Pitfall 2: Composite map keys leaking into filenames or log messages

**What goes wrong:** MDS IR uses `"name@vsanN"` composite keys in the Zones and ZoneConfigs maps. If the converter logs or reports "converted zone `zone_A@vsan10`", the ops team sees the internal key format.
**Why it happens:** Iterating over `cfg.Zones` and using the key in a message instead of `zone.Name`.
**How to avoid:** Always use `zone.Name` (struct field), never the map key, in any user-visible string.

### Pitfall 3: Script file written but not executable

**What goes wrong:** `--script result.sh` creates the file but with default permissions (0666 masked by umask → typically 0644 on Linux/macOS). The ops team tries `./result.sh` and gets "Permission denied".
**Why it happens:** `os.Create` uses 0666 which after umask gives 0644 (no execute bit).
**How to avoid:** Use `os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)` for script files.

### Pitfall 4: Summary counts before vs after sanitization

**What goes wrong:** Summary counts `len(cfg.Aliases)` etc. before sanitization — the count reflects pre-sanitize state. After sanitization with collision disambiguation, counts could differ if two names collide and one is dropped (though current Sanitize keeps all names with disambiguation).
**Why it happens:** Counting at the wrong point in the pipeline.
**How to avoid:** Count AFTER `Sanitize` returns the mutated IR, not before. The `cfg.Warnings` slice captures all sanitizer warnings and should be counted in the final summary.

### Pitfall 5: Cobra `SilenceUsage` not set — usage printed on file-not-found

**What goes wrong:** User runs `san-conv mds2brocade nonexistent.txt` and gets the full `--help` output followed by the error message. This looks like a usage error but it's a runtime error.
**Why it happens:** Cobra defaults to printing usage when `RunE` returns any error.
**How to avoid:** Set `rootCmd.SilenceUsage = true` in `cmd/root.go`'s `init()` function.

### Pitfall 6: Windows script file newlines

**What goes wrong:** Shell script written with `\n` (LF) newlines works on Linux/macOS but Brocade's SSH CLI may be used from Windows tooling. However the _target_ of the script is the Brocade switch SSH session, not Windows — the script runs on the switch.
**Why it happens:** Overthinking cross-platform. The script runs on the Brocade switch, not on Windows.
**How to avoid:** Use `\n` (LF) newlines always in generated FOS scripts. Do not use `\r\n`. This matches how the existing emitters generate output.

---

## Code Examples

### Converter Run Function (verified from existing pipeline signatures)

```go
// internal/converter/converter.go
// Source: internal analysis of existing emitter and parser signatures

func Run(opts Options, stdout io.Writer, stderr io.Writer) error {
    // Open input
    f, err := os.Open(opts.InputFile)
    if err != nil {
        return fmt.Errorf("open %q: %w", opts.InputFile, err)
    }
    defer f.Close()

    // Parse
    var cfg *ir.ZoningConfig
    switch opts.Direction {
    case "mds2brocade":
        cfg, err = mdsparser.Parse(f)
    case "brocade2mds":
        cfg, err = brocadeparser.Parse(f)
    default:
        return fmt.Errorf("unknown direction %q (use mds2brocade or brocade2mds)", opts.Direction)
    }
    if err != nil {
        return fmt.Errorf("parse: %w", err)
    }

    // Sanitize (mds2brocade only — FOS naming rules do not apply to NX-OS output)
    if opts.Direction == "mds2brocade" {
        cfg = validator.Sanitize(cfg, opts.FOSVersion)
    }

    // Resolve primary output writer
    primaryW := stdout
    if opts.OutputFile != "" {
        of, ferr := os.Create(opts.OutputFile)
        if ferr != nil {
            return fmt.Errorf("create output %q: %w", opts.OutputFile, ferr)
        }
        defer of.Close()
        primaryW = of
    }

    // Emit
    switch opts.Direction {
    case "mds2brocade":
        err = brocadeemitter.Emit(cfg, primaryW, false /*scriptMode=false*/)
        if err != nil {
            return fmt.Errorf("emit: %w", err)
        }
        // Emit script if requested (CLI-04)
        if opts.ScriptFile != "" {
            sf, serr := os.OpenFile(opts.ScriptFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
            if serr != nil {
                return fmt.Errorf("create script %q: %w", opts.ScriptFile, serr)
            }
            defer sf.Close()
            if serr = brocadeemitter.Emit(cfg, sf, true /*scriptMode=true*/); serr != nil {
                return fmt.Errorf("emit script: %w", serr)
            }
        }
    case "brocade2mds":
        err = mdsemitter.Emit(cfg, primaryW)
        if err != nil {
            return fmt.Errorf("emit: %w", err)
        }
    }

    // Print warnings and summary to stderr (OUT-04)
    for _, w := range cfg.Warnings {
        fmt.Fprintf(stderr, "WARN: %s\n", w)
    }
    fmt.Fprintf(stderr, "Summary: %d aliases, %d zones, %d configs converted; %d warnings\n",
        len(cfg.Aliases), len(cfg.Zones), len(cfg.ZoneConfigs), len(cfg.Warnings))

    return nil
}
```

### Cobra RunE Wiring (mds2brocade)

```go
// cmd/mds2brocade.go — replace the stub RunE
RunE: func(cmd *cobra.Command, args []string) error {
    outputFile, _ := cmd.Flags().GetString("output")
    scriptFile,  _ := cmd.Flags().GetString("script")
    fosVersion,  _ := cmd.Flags().GetString("fos-version")

    return converter.Run(converter.Options{
        InputFile:  args[0],
        Direction:  "mds2brocade",
        OutputFile: outputFile,
        ScriptFile: scriptFile,
        FOSVersion: fosVersion,
    }, os.Stdout, os.Stderr)
},
```

### Integration Test Pattern

```go
// internal/converter/converter_test.go
func TestRunMDS2Brocade_BasicRoundtrip(t *testing.T) {
    t.Parallel()
    var stdout, stderr bytes.Buffer
    opts := Options{
        InputFile:  "../../testdata/mds/basic.cfg",
        Direction:  "mds2brocade",
        FOSVersion: "pre-8.1",
    }
    err := Run(opts, &stdout, &stderr)
    require.NoError(t, err)
    require.Contains(t, stdout.String(), "alicreate")
    require.Contains(t, stderr.String(), "Summary:")
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `cobra.Command.Run` (no error return) | `cobra.Command.RunE` (returns error) | Foundation phase decision | RunE errors propagate to Execute(), enabling proper exit codes |
| Direct `os.Exit` in business logic | Return error from business logic; only entry point calls os.Exit | Foundation phase | Converter and emitters are testable without process exit |
| Subcommands for direction | Root flag `--direction` (SC#1 implies) | Phase 7 (this phase) | May require collapsing subcommands or adding root-level dispatch |

**Established in codebase:**
- `Run` vs `RunE`: the codebase uses `RunE` on all subcommands — verified in `mds2brocade.go` and `brocade2mds.go`.
- `io.Writer` threading: both emitters accept `io.Writer` — verified in `emitter.go` files.
- `cfg.Warnings` slice: all parsers and emitters append to `cfg.Warnings` — verified in IR definition and emitter code.

---

## Open Questions

1. **Subcommand vs root-level dispatch**
   - What we know: SC#1 says `san-conv myconfig.txt` (no subcommand). SC#4 says `san-conv myconfig.txt --direction brocade2mds`.
   - What's unclear: Whether to keep subcommands (`san-conv mds2brocade myconfig.txt`) and add a root-level default, or collapse to a single root command.
   - Recommendation: The planner should decide. Option B (keep subcommands, change `Args` to `ExactArgs(1)`) is lowest risk. Option C (collapse to root with `--direction`) matches SC#1/#4 literally but is a bigger structural change.

2. **Skipped object count in summary**
   - What we know: OUT-04 says "objects converted, objects skipped, warnings issued." The current emitters do not return a count of skipped zones — they append warnings but don't expose a skip counter.
   - What's unclear: Whether "objects skipped" means "zones skipped because all members were unsupported" or includes sanitizer renames. Can be inferred from warnings count, or a skip counter needs to be added to the IR or emitter return value.
   - Recommendation: Count lines in `cfg.Warnings` that contain "skipped" as the "skipped" count. This avoids changing emitter signatures.

3. **Double-emit side effect for script mode**
   - What we know: The current `brocade.Emit()` appends warnings to `cfg.Warnings` when zones are skipped. If `Run` calls `Emit` twice (once for primary output, once for script), the warnings list will be doubled.
   - What's unclear: Whether this is an actual problem (same warnings appear twice in stderr).
   - Recommendation: Count warnings BEFORE the second emit, or refactor to emit into a `bytes.Buffer` first and write to both outputs. Alternative: Run `Emit` once into a `bytes.Buffer`, then `io.Copy` to both outputs.

---

## Environment Availability

Step 2.6: SKIPPED (no new external dependencies — all tools already verified working in the project).

Confirmed from current session:
- Go 1.26.1 (darwin/arm64) — available
- `go build ./...` — passes (verified)
- `go test ./...` — 51 tests pass across 9 packages (verified)
- goreleaser — present in `.goreleaser.yml`, goreleaser binary expected at `$(GORELEASER)` in Makefile

---

## Validation Architecture

`workflow.nyquist_validation` is `false` in `.planning/config.json`. This section is skipped.

---

## Sources

### Primary (HIGH confidence)
- Codebase direct read: `cmd/root.go`, `cmd/mds2brocade.go`, `cmd/brocade2mds.go` — cobra command structure, existing flag stubs, RunE pattern
- Codebase direct read: `internal/emitter/brocade/emitter.go` — `Emit(cfg *ir.ZoningConfig, w io.Writer, scriptMode bool) error` signature
- Codebase direct read: `internal/emitter/mds/emitter.go` — `Emit(cfg *ir.ZoningConfig, w io.Writer) error` signature
- Codebase direct read: `internal/parser/mds/parser.go` — `Parse(r io.Reader) (*ir.ZoningConfig, error)` signature
- Codebase direct read: `internal/parser/brocade/parser.go` — `Parse(r io.Reader) (*ir.ZoningConfig, error)` signature
- Codebase direct read: `internal/validator/sanitizer.go` — `Sanitize(cfg *ir.ZoningConfig, fosVersion string) *ir.ZoningConfig` signature
- Codebase direct read: `internal/converter/doc.go` — "Implemented in Phase 7" comment confirms this is the right package
- Codebase direct read: `internal/ir/zoningconfig.go` — `Warnings []string` field on `ZoningConfig`
- Codebase direct read: `.planning/REQUIREMENTS.md` — CLI-01 through CLI-06 and OUT-04 definitions
- Codebase direct read: `Makefile` — `run-mds` and `run-brocade` targets showing expected invocation pattern
- `go test ./...` execution — 51 tests pass, build clean

### Secondary (MEDIUM confidence)
- Cobra documentation (known from training, consistent with codebase usage): `RunE` vs `Run`, `SilenceUsage`, `cobra.ExactArgs`, `cmd.Flags().GetString()`
- Go stdlib `os.OpenFile` with 0755 permissions — standard pattern for executable script creation

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new packages; all existing packages already in use
- Architecture: HIGH — pipeline signatures are all verified from source; converter.go is the only new file
- Pitfalls: HIGH — all pitfalls derived from direct code inspection (sanitizer direction scope, composite key leakage, script permissions)
- Open questions: MEDIUM — subcommand vs root-level design is a planner decision, not a research gap

**Research date:** 2026-03-29
**Valid until:** 2026-06-01 (stable ecosystem — no fast-moving dependencies)
