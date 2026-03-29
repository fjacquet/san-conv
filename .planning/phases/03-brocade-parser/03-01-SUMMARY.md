---
phase: 03-brocade-parser
plan: 01
subsystem: testing
tags: [go, brocade, fos, cfgshow, cli, tdd, fixtures, parser]

# Dependency graph
requires:
  - phase: 02-mds-parser
    provides: MDS parser_test.go structure and fixture patterns that Brocade tests mirror
  - phase: 01-foundation
    provides: ir.ZoningConfig IR struct that tests assert against

provides:
  - Five Brocade FOS test fixture files covering cfgshow basic, continuation, CLI basic, CLI pWWN, edge cases
  - parser_test.go with 5 table-driven tests establishing the parser contract (TDD red phase)

affects: [03-brocade-parser, 04-mds-emitter, 05-brocade-emitter]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Brocade fixture files: cfgshow Defined configuration: section with alias:/zone:/cfg: tokens"
    - "Brocade fixture files: backslash continuation for multi-line member lists"
    - "Brocade fixture files: Effective configuration: as section boundary (must NOT be parsed)"
    - "Brocade fixture files: CLI format alicreate/zonecreate/cfgcreate one-command-per-line"
    - "TDD red phase: parser_test.go references Parse() that does not exist yet; go vet confirms undefined: Parse"

key-files:
  created:
    - testdata/brocade/cfgshow_basic.cfg
    - testdata/brocade/cfgshow_continuation.cfg
    - testdata/brocade/cli_basic.cfg
    - testdata/brocade/cli_pwwn_members.cfg
    - testdata/brocade/edge_cases.cfg
    - internal/parser/brocade/parser_test.go
  modified: []

key-decisions:
  - "Five fixture files use plain zone names as map keys (not name@vsan0) consistent with Brocade plain-name key strategy"
  - "Backslash continuation fixture uses trailing backslash with space (member_01; member_02; \\) to test continuation parser"
  - "edge_cases.cfg includes Effective configuration: section to enforce section-boundary test (Defined only must be parsed)"
  - "TDD red phase: parser_test.go references Parse() before it is implemented to enforce test-first development"

patterns-established:
  - "Brocade test fixture naming: cfgshow_*.cfg for cfgshow format, cli_*.cfg for CLI format, edge_cases.cfg for boundary tests"
  - "Brocade VSAN sentinel: all VSAN fields asserted as 0 in tests (no VSAN concept in Brocade)"
  - "Map key strategy: plain zone/cfg name (not name@vsan0) for Brocade IR — decided in Phase 3 fixtures"

requirements-completed: [PARSE-07, PARSE-08, PARSE-09]

# Metrics
duration: 4min
completed: 2026-03-29
---

# Phase 03 Plan 01: Brocade Parser TDD Red Phase Summary

**Five Brocade FOS fixture files plus table-driven parser_test.go establishing the cfgshow/CLI parse contract before implementation**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-29T08:00:57Z
- **Completed:** 2026-03-29T08:05:00Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Created five Brocade FOS test fixtures covering all three requirements (PARSE-07 cfgshow, PARSE-08 CLI, PARSE-09 auto-detection)
- Created parser_test.go with 5 table-driven subtests with specific IR assertions that define the exact parser contract
- TDD red phase established: `go vet ./internal/parser/brocade/` reports `undefined: Parse` confirming tests reference unimplemented function
- Removed .gitkeep from testdata/brocade/ as five real fixtures now exist

## Task Commits

1. **Task 1: Create five Brocade test fixture files** - `9a553c2` (feat)
2. **Task 2: Create parser_test.go with table-driven tests** - `07e8f74` (test)

**Plan metadata:** (committed with STATE.md update)

## Files Created/Modified

- `testdata/brocade/cfgshow_basic.cfg` - Basic cfgshow with 4 aliases, 2 zones, 1 cfg (PARSE-07, PARSE-09)
- `testdata/brocade/cfgshow_continuation.cfg` - Backslash continuation: big_zone has 4 members across 2 lines (PARSE-07)
- `testdata/brocade/cli_basic.cfg` - FOS CLI script: alicreate/zonecreate/cfgcreate (PARSE-08, PARSE-09)
- `testdata/brocade/cli_pwwn_members.cfg` - CLI zonecreate with raw pWWN members (PARSE-08)
- `testdata/brocade/edge_cases.cfg` - Empty zone + Effective configuration: section boundary (PARSE-07 edge case)
- `internal/parser/brocade/parser_test.go` - 5 table-driven tests with IR assertions and TDD red phase confirmation

## Decisions Made

- Fixtures use plain zone/cfg names as map keys (not `name@vsan0`): consistent with Brocade IR key strategy where VSAN 0 is always the sentinel and map keys need no VSAN disambiguation
- edge_cases.cfg includes `Effective configuration:` section explicitly to create a test that asserts the parser stops at that boundary and does not double-count entries
- backslash continuation test uses `big_zone` with 4 members split across 2 lines — this is the critical regression test for the continuation parser
- All VSAN assertions are `== 0` throughout all test cases confirming Brocade VSAN sentinel

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All five fixture files are in place, ready for Plan 03-02 (parser.go implementation)
- parser_test.go defines the exact IR contract the implementation must satisfy
- TDD red phase confirmed: `go vet` reports `undefined: Parse`
- Plan 03-02 (green phase) will implement `internal/parser/brocade/parser.go` with the `Parse(r io.Reader)` function

---
*Phase: 03-brocade-parser*
*Completed: 2026-03-29*

## Self-Check: PASSED
