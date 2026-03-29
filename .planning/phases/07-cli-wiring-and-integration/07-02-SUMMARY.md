---
phase: 07-cli-wiring-and-integration
plan: 02
subsystem: cli
tags: [go, cobra, converter, pipeline, cli, integration]

# Dependency graph
requires:
  - phase: 07-01
    provides: TDD red phase — 10 failing converter_test.go tests for Options and Run()
  - phase: 06-mds-emitter
    provides: MDS NX-OS emitter (Emit function)
  - phase: 05-brocade-emitter
    provides: Brocade FOS emitter (Emit function with scriptMode)
  - phase: 04-validator-and-sanitizer
    provides: Sanitize function for FOS naming rules
  - phase: 03-brocade-parser
    provides: Brocade Parse function (auto-detects cfgshow vs CLI script)
  - phase: 02-mds-parser
    provides: MDS Parse function
provides:
  - internal/converter/converter.go — Options struct and Run() pipeline orchestrator
  - cmd/mds2brocade.go — cobra RunE wired to converter.Run with all flags
  - cmd/brocade2mds.go — cobra RunE wired to converter.Run with output flag
  - cmd/root.go — SilenceUsage=true set
  - Working san-conv binary with fully operational mds2brocade and brocade2mds subcommands
affects:
  - All future phases that test end-to-end CLI operation

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "converter.Run() accepts io.Writer for stdout and stderr — testable without stdout capture"
    - "Script file created with os.OpenFile(..., 0755) not os.Create — executable permission"
    - "Warning deduplication: snapshot len(cfg.Warnings) before second brocadeemitter.Emit call"
    - "SilenceUsage=true on rootCmd suppresses usage output on runtime errors (keeps stderr clean)"
    - "ExactArgs(1) enforced on both subcommands — missing file caught by cobra before RunE"

key-files:
  created:
    - internal/converter/converter.go
  modified:
    - cmd/mds2brocade.go
    - cmd/brocade2mds.go
    - cmd/root.go

key-decisions:
  - "converter.Run() takes io.Writer parameters for stdout/stderr — not os.Stdout/os.Stderr directly — so tests can use bytes.Buffer"
  - "Sanitize() called only in mds2brocade branch — brocade2mds preserves hyphens in names unchanged"
  - "Script emit uses a bytes.Buffer intermediate then writes to file — avoids partial writes and simplifies error handling"
  - "Warning count snapshotted before second brocadeemitter.Emit to prevent doubling warnings in stderr Summary"
  - "SilenceUsage=true set in root.go init() — ops team sees only the error message on missing file, not multi-line usage"

patterns-established:
  - "Pipeline pattern: parse → sanitize (mds2brocade only) → emit primary → emit script (optional) → warnings + summary"
  - "ExactArgs(1) on CLI subcommands that require a file — cobra validates before RunE is called"

requirements-completed:
  - CLI-01
  - CLI-02
  - CLI-03
  - CLI-04
  - CLI-05
  - CLI-06
  - OUT-04

# Metrics
duration: 4min
completed: 2026-03-29
---

# Phase 07 Plan 02: CLI Wiring and Integration — Green Phase Summary

**converter.Run() pipeline orchestrator implemented and cobra stubs wired — san-conv binary fully operational with mds2brocade and brocade2mds subcommands, all 61 tests passing**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-29T15:46:40Z
- **Completed:** 2026-03-29T15:51:01Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments

- Created `internal/converter/converter.go` with `Options` struct and `Run()` function implementing the full parse → validate → emit pipeline
- Wired `cmd/mds2brocade.go` and `cmd/brocade2mds.go` to call `converter.Run()` with ExactArgs(1) validation
- Set `SilenceUsage = true` in `cmd/root.go` to suppress usage text on runtime errors
- All 10 converter integration tests from Plan 07-01 now pass; full suite at 61/61 tests
- End-to-end smoke tests verified: alicreate/zonecreate/cfgcreate output, 0755 script file, device-alias output, non-zero exit on missing file

## Task Commits

1. **Task 1: Implement converter.go pipeline orchestrator** - `89cdea3` (feat)
2. **Task 2: Wire cobra commands and set SilenceUsage** - `c715547` (feat)

## Files Created/Modified

- `internal/converter/converter.go` — Options struct, Run() pipeline orchestrator (parse → validate → emit → summary)
- `cmd/mds2brocade.go` — ExactArgs(1), RunE calls converter.Run with output/script/fos-version flags
- `cmd/brocade2mds.go` — ExactArgs(1), RunE calls converter.Run with output flag
- `cmd/root.go` — SilenceUsage=true added to init()

## Decisions Made

- `converter.Run()` accepts `io.Writer` parameters (not `os.Stdout`/`os.Stderr` directly) — this is the pattern that makes all 10 integration tests work without output capture hacks
- `Sanitize()` is called only in the `mds2brocade` branch — `brocade2mds` preserves hyphens and all original names unchanged (Test 6 validates this)
- Warning count is snapshotted before the script emit call (`warnCountBefore := len(cfg.Warnings)`) to prevent the second `brocadeemitter.Emit` call from doubling warnings in stderr (Test 4 validates this)
- Script file uses `os.OpenFile(..., 0755)` not `os.Create` — executable permission is required and tested (Test 3 validates with `mode & 0o111`)

## Deviations from Plan

None — plan executed exactly as written. All interfaces matched the specifications in the plan's `<interfaces>` section.

## Issues Encountered

`golangci-lint run ./...` exits with errors in `charmbracelet/x/cellbuf` (a transitive dependency of golangci-lint itself), not in project code. This is a pre-existing incompatibility in the linter's own dependencies. `go vet ./...` and `go build ./...` both pass cleanly. No lint errors found in project code.

## Next Phase Readiness

- Phase 07 is complete: all parsers, validator, emitters, and CLI are wired together
- `san-conv` binary produces correct, ready-to-apply FOS CLI commands from MDS configs
- Binary accepts `--output` and `--script` flags; `--fos-version` controls sanitizer behavior
- All CLI requirements (CLI-01 through CLI-06) and OUT-04 are satisfied
- Project is ready for goreleaser packaging and distribution

---
*Phase: 07-cli-wiring-and-integration*
*Completed: 2026-03-29*
