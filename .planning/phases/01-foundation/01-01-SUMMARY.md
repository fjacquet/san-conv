---
phase: 01-foundation
plan: "01"
subsystem: infra
tags: [go, cobra, golangci-lint, goreleaser, ir, module-init]

# Dependency graph
requires: []
provides:
  - Go module github.com/fjacquet/san-conv with cobra v1.10.2 and testify v1.11.1
  - IR contract: ZoningConfig, Alias, Zone, ZoneMember, ZoneConfig structs in internal/ir
  - Compilable cmd package with mds2brocade and brocade2mds stub subcommands
  - Six empty internal sub-packages reserving namespaces for Phases 2-7
  - testdata/mds/ and testdata/brocade/ directories for fixture files
affects: [02-mds-parser, 03-brocade-parser, 04-validator, 05-brocade-emitter, 06-mds-emitter, 07-cli-wiring]

# Tech tracking
tech-stack:
  added:
    - github.com/spf13/cobra v1.10.2 (CLI framework)
    - github.com/stretchr/testify v1.11.1 (test assertions)
    - github.com/golangci/golangci-lint/v2 v2.11.4 (go.mod tool directive)
    - github.com/goreleaser/goreleaser/v2 v2.14.3 (go.mod tool directive)
    - github.com/spf13/cobra-cli v1.3.0 (go.mod tool directive)
  patterns:
    - IR-first design: ZoningConfig is the shared data contract; all parsers produce it, all emitters consume it
    - Compiler pipeline: parser -> IR -> validator -> emitter with strict import DAG (no cross-package imports)
    - cobra RunE stubs returning fmt.Errorf("not yet implemented") for correct non-zero exit
    - Go 1.24+ tool directive instead of tools.go blank imports

key-files:
  created:
    - go.mod (module github.com/fjacquet/san-conv, go 1.26.1, tool block with three dev tools)
    - go.sum (2192 entries)
    - main.go (entry point calling cmd.Execute())
    - cmd/root.go (cobra root command with Execute() function)
    - cmd/mds2brocade.go (stub subcommand with flags declared for --help)
    - cmd/brocade2mds.go (stub subcommand)
    - internal/ir/zoningconfig.go (five IR structs, zero imports)
    - internal/parser/mds/doc.go (empty stub)
    - internal/parser/brocade/doc.go (empty stub)
    - internal/validator/doc.go (empty stub)
    - internal/emitter/brocade/doc.go (empty stub)
    - internal/emitter/mds/doc.go (empty stub)
    - internal/converter/doc.go (empty stub)
    - testdata/mds/.gitkeep
    - testdata/brocade/.gitkeep
  modified:
    - go.mod (cobra promoted to direct dep after cmd/ source files created)

key-decisions:
  - "golangci-lint v2 module path is github.com/golangci/golangci-lint/v2/cmd/golangci-lint (not /cmd/golangci-lint without /v2 suffix)"
  - "goreleaser v2 module path is github.com/goreleaser/goreleaser/v2 (not /goreleaser without /v2 suffix)"
  - "IR package has zero imports by design — prevents all import cycles across the compiler pipeline"
  - "testify remains indirect in go.mod until test files exist — expected behavior, not an issue"

patterns-established:
  - "Pattern 1: IR-zero-imports — internal/ir/zoningconfig.go imports nothing; it is the cycle-free root of the dependency DAG"
  - "Pattern 2: RunE stubs — cobra subcommands use RunE returning fmt.Errorf to guarantee non-zero exit when unimplemented"
  - "Pattern 3: doc.go stubs — empty sub-packages declare package name in doc.go to register namespace with Go toolchain"
  - "Pattern 4: go.mod tool directive — dev tools tracked via tool block, not tools.go hack"

requirements-completed:
  - CLI-07

# Metrics
duration: 8min
completed: 2026-03-29
---

# Phase 1 Plan 01: Foundation — Go Module Init Summary

**Go module github.com/fjacquet/san-conv scaffolded with cobra CLI stubs, complete IR contract (5 structs, zero imports), and six empty internal package stubs unblocking all downstream phases**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-29T04:51:57Z
- **Completed:** 2026-03-29T05:00:00Z
- **Tasks:** 2 of 2
- **Files modified:** 15

## Accomplishments

- Initialized Go module github.com/fjacquet/san-conv with go 1.26.1, cobra v1.10.2, testify v1.11.1, and three dev tool directives (golangci-lint, goreleaser, cobra-cli)
- Defined complete IR contract in internal/ir/zoningconfig.go: ZoningConfig, Alias, Zone, ZoneMember, ZoneConfig with zero imports (import-cycle-free root of compiler pipeline DAG)
- Created cobra CLI skeleton: main.go → cmd.Execute() → root/mds2brocade/brocade2mds stubs with RunE returning not-yet-implemented errors
- Created six empty internal sub-package stubs (doc.go) reserving namespaces for Phases 2-7: parser/mds, parser/brocade, validator, emitter/brocade, emitter/mds, converter
- go build ./... and go test ./... both pass with zero errors

## Task Commits

Each task was committed atomically:

1. **Task 1: Initialize Go module and install dependencies** - `557dc61` (chore)
2. **Task 2: Create IR structs, main.go, and empty package stubs** - `324b719` (feat)

**Plan metadata:** (final docs commit — hash TBD)

## Files Created/Modified

- `go.mod` - Module declaration with tool block for golangci-lint, goreleaser, cobra-cli
- `go.sum` - 2192 dependency checksums
- `main.go` - Entry point calling cmd.Execute()
- `cmd/root.go` - Cobra root command with Execute() function
- `cmd/mds2brocade.go` - Stub subcommand with --output, --script, --fos-version flags
- `cmd/brocade2mds.go` - Stub subcommand with --output flag
- `internal/ir/zoningconfig.go` - Five IR structs with zero imports (the shared data contract)
- `internal/parser/mds/doc.go` - Phase 2 namespace stub
- `internal/parser/brocade/doc.go` - Phase 3 namespace stub
- `internal/validator/doc.go` - Phase 4 namespace stub
- `internal/emitter/brocade/doc.go` - Phase 5 namespace stub
- `internal/emitter/mds/doc.go` - Phase 6 namespace stub
- `internal/converter/doc.go` - Phase 7 namespace stub
- `testdata/mds/.gitkeep` - Fixture directory placeholder
- `testdata/brocade/.gitkeep` - Fixture directory placeholder

## Decisions Made

- Used `github.com/golangci/golangci-lint/v2/cmd/golangci-lint` module path (v2 uses /v2 major version suffix per Go module conventions — the plan's suggested path was incorrect but auto-corrected)
- Used `github.com/goreleaser/goreleaser/v2` module path (same reason — v2 suffix required)
- IR package designed with zero imports to prevent any possibility of import cycles; this is a hard constraint for the compiler pipeline pattern
- testify kept as indirect dependency in go.mod (expected — no test files yet; will become direct when test files are added in Phase 2+)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected golangci-lint tool directive module path**
- **Found during:** Task 1 (go get -tool command)
- **Issue:** Plan specified `github.com/golangci/golangci-lint/cmd/golangci-lint@v2.11.4` but this path fails because v2 uses the `/v2` major version module suffix required by Go module conventions
- **Fix:** Used `github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4` (correct v2 path)
- **Files modified:** go.mod, go.sum
- **Verification:** `go mod verify` passes; tool directive present in go.mod
- **Committed in:** 557dc61 (Task 1 commit)

**2. [Rule 1 - Bug] Corrected goreleaser tool directive module path**
- **Found during:** Task 1 (go get -tool command)
- **Issue:** Plan specified `github.com/goreleaser/goreleaser@v2.14.3` but v2 requires `/v2` suffix
- **Fix:** Used `github.com/goreleaser/goreleaser/v2@v2.14.3` (correct v2 path)
- **Files modified:** go.mod, go.sum
- **Verification:** `go mod verify` passes; tool directive present in go.mod
- **Committed in:** 557dc61 (Task 1 commit)

---

**Total deviations:** 2 auto-fixed (2 Rule 1 - Bug corrections)
**Impact on plan:** Both fixes were necessary to install the correct v2 module paths. The underlying tools (golangci-lint v2.11.4, goreleaser v2.14.3) are exactly as planned — only the Go module import paths needed the standard /v2 suffix correction. No scope creep.

## Issues Encountered

None beyond the module path corrections documented above.

## Known Stubs

The following stubs are intentional placeholders for downstream phases:

| File | Stub Type | Reason | Resolved In |
|------|-----------|--------|-------------|
| `cmd/mds2brocade.go` | RunE returns not-yet-implemented error | Phase 7 will wire parser → converter → emitter | Phase 7 (CLI Wiring) |
| `cmd/brocade2mds.go` | RunE returns not-yet-implemented error | Phase 7 will wire parser → converter → emitter | Phase 7 (CLI Wiring) |
| `internal/parser/mds/doc.go` | Empty package stub | Parser implementation deferred | Phase 2 (MDS Parser) |
| `internal/parser/brocade/doc.go` | Empty package stub | Parser implementation deferred | Phase 3 (Brocade Parser) |
| `internal/validator/doc.go` | Empty package stub | Validator implementation deferred | Phase 4 (Validator) |
| `internal/emitter/brocade/doc.go` | Empty package stub | Emitter implementation deferred | Phase 5 (Brocade Emitter) |
| `internal/emitter/mds/doc.go` | Empty package stub | Emitter implementation deferred | Phase 6 (MDS Emitter) |
| `internal/converter/doc.go` | Empty package stub | Pipeline wiring deferred | Phase 7 (Converter) |

These stubs are the planned output of Phase 1 — they unblock parallel development of Phases 2-7 without affecting Phase 1's goal (establishing the module and IR contract).

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- IR contract is locked and stable — Phases 2 (MDS Parser) and 3 (Brocade Parser) can begin immediately
- Module path and dependency versions are established — no changes needed
- Go build and test infrastructure verified working
- Blockers carried over from STATE.md:
  - Multi-VSAN output strategy (one merged file vs per-VSAN files) needs ops team input before Phase 2 implementation
  - Test fixture availability: real NX-OS 8.5+ enhanced device-alias configs need to be sourced or synthesized before Phase 2

---
*Phase: 01-foundation*
*Completed: 2026-03-29*

## Self-Check: PASSED

All files verified present:
- go.mod, go.sum, main.go, cmd/root.go
- internal/ir/zoningconfig.go
- All 6 doc.go stubs (parser/mds, parser/brocade, validator, emitter/brocade, emitter/mds, converter)
- testdata/mds/.gitkeep, testdata/brocade/.gitkeep
- .planning/phases/01-foundation/01-01-SUMMARY.md

All commits verified present:
- 557dc61 (Task 1: Go module init)
- 324b719 (Task 2: IR structs and package stubs)
