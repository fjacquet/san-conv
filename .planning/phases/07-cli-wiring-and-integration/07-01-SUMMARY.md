---
phase: 07-cli-wiring-and-integration
plan: 01
subsystem: testing
tags: [go, tdd, converter, cli, integration-tests, testify]

# Dependency graph
requires:
  - phase: 06-mds-emitter
    provides: MDS NX-OS emitter (Emit function signature verified)
  - phase: 05-brocade-emitter
    provides: Brocade FOS emitter (Emit function signature verified)
  - phase: 04-validator-and-sanitizer
    provides: Sanitize function signature verified
  - phase: 03-brocade-parser
    provides: Brocade Parse function signature verified
  - phase: 02-mds-parser
    provides: MDS Parse function signature verified
provides:
  - TDD RED phase: 10 failing integration tests for converter.Run()
  - Behavioral contract for Options struct and Run() function
  - Full coverage of CLI requirements CLI-01 through CLI-06 and OUT-04
affects:
  - 07-02-PLAN.md (must implement converter.go to make these tests pass)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "TDD red phase: test file in same package (white-box), converter.go not yet created"
    - "bytes.Buffer for stdout/stderr capture in integration tests"
    - "t.TempDir() for isolated output file paths in tests"
    - "os.WriteFile for inline fixture creation when testdata doesn't have needed variant"
    - "os.Stat mode & 0o111 to assert executable permission on script files"

key-files:
  created:
    - internal/converter/converter_test.go
  modified: []

key-decisions:
  - "Test 6 uses inline fixture via os.WriteFile to a temp file — no existing brocade fixture has hyphenated alias names"
  - "All 10 tests use t.Parallel() for concurrent execution"
  - "package converter (not package converter_test) for white-box access when converter.go exists"
  - "Summary: pattern check uses regexp not strings.Contains to enforce format rigorously"

patterns-established:
  - "Integration tests call converter.Run(opts, &stdout, &stderr) — never call internal packages directly"
  - "Inline fixture pattern: os.WriteFile to t.TempDir() when testdata lacks a specific variant"

requirements-completed:
  - CLI-01
  - CLI-02
  - CLI-03
  - CLI-04
  - CLI-05
  - CLI-06
  - OUT-04

# Metrics
duration: 2min
completed: 2026-03-29
---

# Phase 07 Plan 01: CLI Wiring and Integration — TDD Red Phase Summary

**10 failing integration tests encoding the full converter.Run() behavioral contract: CLI flags, output routing, script permission, summary format, and direction semantics**

## Performance

- **Duration:** 2 min
- **Started:** 2026-03-29T15:41:33Z
- **Completed:** 2026-03-29T15:43:22Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Created `internal/converter/converter_test.go` with 10 named, parallel integration tests
- Tests reference `converter.Options` and `converter.Run()` which are undefined — RED phase confirmed (`go test ./internal/converter/...` fails with `undefined: Options` and `undefined: Run`)
- Tests cover all 7 CLI/output requirements: basic stdout, file output (stdout empty), script file with executable permission, no double-Summary in stderr, brocade2mds output, hyphen preservation in brocade2mds, FOSVersion flag acceptance, missing file error, unknown direction error, and summary format regexp

## Task Commits

1. **Task 1: converter.Run integration tests (TDD red phase)** - `1ab3154` (test)

## Files Created/Modified

- `internal/converter/converter_test.go` — 10 integration tests for Options struct and Run() function

## Decisions Made

- Test 6 (hyphen preservation) uses an inline fixture via `os.WriteFile` to `t.TempDir()` since no existing brocade testdata fixture contains hyphenated alias names — this keeps the test self-contained without polluting testdata
- All tests use `package converter` (not `_test` suffix) for white-box access, allowing tests to compile once `converter.go` defines the exported symbols
- `require.Regexp` used for summary format check (test 10) instead of `require.Contains` — enforces the exact `"N aliases, N zones, N configs converted; N warnings"` structure

## Deviations from Plan

None — plan executed exactly as written. All 10 specified tests implemented with the required assertions.

## Issues Encountered

None.

## Next Phase Readiness

- `internal/converter/converter_test.go` defines the complete behavioral contract
- Plan 07-02 must create `internal/converter/converter.go` with `Options` struct and `Run()` function
- All test fixture paths are verified to exist in `testdata/mds/` and `testdata/brocade/`
- No new Go dependencies needed — all imports are stdlib or testify (already in go.mod)

---
*Phase: 07-cli-wiring-and-integration*
*Completed: 2026-03-29*
