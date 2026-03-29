---
phase: 05-brocade-emitter
plan: 01
subsystem: emitter
tags: [go, brocade, fos, tdd, red-phase, emitter, io.Writer, table-driven-tests]

# Dependency graph
requires:
  - phase: 04-validator-and-sanitizer
    provides: sanitized *ir.ZoningConfig with clean FOS-compatible names
  - phase: 01-foundation
    provides: ir package with ZoningConfig, Alias, Zone, ZoneConfig, ZoneMember structs
provides:
  - Complete behavioral contract for Emit() function via 10 table-driven tests
  - TDD red phase: emitter_test.go failing with "undefined: Emit"
affects: [05-02-brocade-emitter-green, 06-mds-emitter, 07-cli-wiring]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TDD red phase: test file references unimplemented Emit() — go vet confirms undefined: Emit"
    - "Table-driven tests with checkFn(t, output, cfg) signature for stateful warning assertions"
    - "bytes.Buffer as io.Writer in tests for output capture without file I/O"
    - "Inline IR construction in test cases — no fixture files needed for emitter tests"

key-files:
  created:
    - internal/emitter/brocade/emitter_test.go
  modified: []

key-decisions:
  - "checkFn signature includes cfg *ir.ZoningConfig to allow asserting on cfg.Warnings after Emit"
  - "Test 8 (empty zone) asserts both on output absence AND cfg.Warnings content — validates warn-and-continue behavior"
  - "Test 9 (multi-VSAN) uses composite map keys 'zoneA@vsan10' matching MDS parser output format"
  - "Test 10 (deterministic) uses 5 deliberately unsorted alias names to stress Go map ordering"

patterns-established:
  - "Pattern: helper functions makeAlias(name, pwwn), makeZone, makeZoneVSAN, makeZoneConfig, makeMember, makeCfg — mirrors sanitizer_test.go pattern"
  - "Pattern: t.Parallel() at both TestEmit and sub-test level for parallel test execution"

requirements-completed: [CONV-01, CONV-02, CONV-03, OUT-01, OUT-02]

# Metrics
duration: 7min
completed: 2026-03-29
---

# Phase 5 Plan 01: Brocade Emitter TDD Red Phase Summary

**10 table-driven tests defining the complete Brocade FOS emitter behavioral contract — all failing with undefined: Emit (TDD red phase)**

## Performance

- **Duration:** 7 min
- **Started:** 2026-03-29T12:27:47Z
- **Completed:** 2026-03-29T12:35:13Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Created emitter_test.go with 10 table-driven tests covering all 5 phase requirements (CONV-01, CONV-02, CONV-03, OUT-01, OUT-02)
- TDD red phase confirmed: `go vet ./internal/emitter/brocade/` fails with exactly "undefined: Emit"
- Tests cover the full emitter behavioral contract: alicreate/zonecreate/cfgcreate emission, correct ordering, script mode preamble/postamble, cfgenable-always-commented, empty zone skip with warning, multi-VSAN MDS IR flattening, and deterministic output

## Task Commits

Each task was committed atomically:

1. **Task 1: Create emitter_test.go with 10 table-driven tests covering all requirements** - `9b6f2c0` (test)

## Files Created/Modified

- `internal/emitter/brocade/emitter_test.go` — Complete 341-line test suite defining the emitter behavioral contract; all tests fail with "undefined: Emit"

## Decisions Made

- **checkFn signature includes cfg parameter:** `checkFn(t, output, cfg)` instead of just `checkFn(t, output)` — needed so Test 8 (empty zone skip) can assert on `cfg.Warnings` after Emit modifies it
- **Test 9 multi-VSAN uses composite map keys:** Zone keys like `"zoneA@vsan10"` with `Zone.Name = "zoneA"` mirror exact MDS parser output; tests verify emitter uses `zone.Name` not map key
- **Deterministic test uses 5 unsorted aliases:** Names "eee", "aaa", "ddd", "bbb", "ccc" entered in non-alphabetical order stress-test the sort behavior that Plan 02 must implement

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Next Phase Readiness

- emitter_test.go provides the complete behavioral specification for Plan 02
- Plan 02 must implement `Emit(cfg *ir.ZoningConfig, w io.Writer, scriptMode bool) error` in `internal/emitter/brocade/emitter.go`
- All 10 tests will pass once Plan 02 is complete
- Key implementation notes for Plan 02:
  - Use `fmt.Fprintf(w, "alicreate \"%s\", \"%s\"\n", name, pwwn)` — NOT `%q` (avoids Go escape sequences)
  - Sort all map keys before iterating (sort.Strings) for deterministic output
  - Use `zone.Name` field (not map key) when emitting zonecreate commands
  - Track emitted zones set; filter cfgcreate ZoneNames to only emitted zones

---
*Phase: 05-brocade-emitter*
*Completed: 2026-03-29*
