---
phase: 06-mds-emitter
plan: 01
subsystem: testing
tags: [go, mds, nxos, emitter, tdd, device-alias, zoneset]

# Dependency graph
requires:
  - phase: 05-brocade-emitter
    provides: Brocade emitter test structure (emitter_test.go pattern to mirror)
  - phase: 01-foundation
    provides: internal/ir/zoningconfig.go IR struct definitions
provides:
  - Complete behavioral contract for MDS Emit(*ir.ZoningConfig, io.Writer) error via 10 table-driven tests
  - TDD red phase: go vet confirms undefined: Emit
affects:
  - 06-02-PLAN.md (green phase implementation driven by these tests)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "MDS emitter tests use 2-arg Emit(cfg, w) — no scriptMode (NX-OS has single paste-config format)"
    - "VSAN 0 sentinel resolution: tests assert vsan 0 never appears, vsan 1 is the default"
    - "zoneset activate is NOT commented in NX-OS (unlike Brocade cfgenable) — test enforces this"
    - "makeZoneConfigVSAN helper added for multi-VSAN test case"

key-files:
  created:
    - internal/emitter/mds/emitter_test.go
  modified: []

key-decisions:
  - "MDS Emit() has no scriptMode parameter — NX-OS paste-config has a single format (no preamble/postamble distinction)"
  - "Test 6 asserts vsan 0 never appears in output — VSAN 0 sentinel must always resolve to vsan 1"
  - "Test 4 verifies zoneset activate is emitted as a real (non-commented) command — unlike Brocade cfgenable"
  - "Test 8 uses makeZoneConfigVSAN helper to set ZoneConfig.VSAN explicitly for multi-VSAN passthrough scenario"

patterns-established:
  - "Pattern: checkFn signature includes cfg *ir.ZoningConfig to allow asserting on cfg.Warnings after Emit"
  - "Pattern: Test 7 (empty zone) asserts both output absence AND cfg.Warnings content"
  - "Pattern: Test 8 multi-VSAN uses composite map keys 'zoneA@vsan10' with Zone.Name='zoneA'"

requirements-completed:
  - CONV-04
  - CONV-05
  - CONV-06
  - OUT-03

# Metrics
duration: 8min
completed: 2026-03-29
---

# Phase 6 Plan 01: MDS Emitter TDD Red Phase Summary

**10 table-driven failing tests defining MDS Emit(cfg, w) behavioral contract — covers device-alias block, zone blocks with VSAN sentinel, zoneset+activate, and determinism**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-29T15:14:30Z
- **Completed:** 2026-03-29T15:22:30Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Created `internal/emitter/mds/emitter_test.go` with 10 table-driven tests (339 lines)
- Confirmed TDD red phase: `go vet ./internal/emitter/mds/` reports `undefined: Emit`
- All 4 requirement IDs (CONV-04, CONV-05, CONV-06, OUT-03) referenced in test names
- Key behavioral contracts established: no scriptMode, VSAN 0 sentinel resolution, zoneset activate is real (not commented), deterministic output

## Task Commits

1. **Task 1: Create emitter_test.go with 10 table-driven tests** - `12a346e` (test)

**Plan metadata:** _(to be added in final commit)_

## Files Created/Modified

- `internal/emitter/mds/emitter_test.go` - Complete behavioral contract for MDS Emit() via 10 table-driven tests; TDD red phase confirms undefined: Emit

## Decisions Made

- MDS `Emit()` uses 2-argument signature `Emit(cfg *ir.ZoningConfig, w io.Writer) error` — no `scriptMode` (NX-OS paste-config is a single format with no script wrapper distinction)
- `zoneset activate name X vsan N` is a real non-commented command in NX-OS — tests verify it is NOT commented out (unlike Brocade `cfgenable`)
- VSAN 0 sentinel must resolve to VSAN 1 in all emitted output — Test 6 enforces this with `require.NotContains(t, output, "vsan 0")`
- Added `makeZoneConfigVSAN` helper to support Test 8 multi-VSAN scenario where ZoneConfig.VSAN is explicitly set

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None — no external service configuration required.

## Next Phase Readiness

- emitter_test.go complete with all 10 failing tests
- Plan 02 (green phase) can now implement `internal/emitter/mds/emitter.go` to make all tests pass
- Implementation skeleton provided in 06-RESEARCH.md — follow it closely for the Emit() function body

## Self-Check: PASSED

- `internal/emitter/mds/emitter_test.go` — FOUND
- Commit `12a346e` — FOUND

---
*Phase: 06-mds-emitter*
*Completed: 2026-03-29*
