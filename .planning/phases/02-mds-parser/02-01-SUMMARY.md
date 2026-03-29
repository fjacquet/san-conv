---
phase: 02-mds-parser
plan: 01
subsystem: testing
tags: [nxos, mds, fixtures, testdata, zoning, device-alias, fcalias, zoneset, vsan]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: IR structs (internal/ir/zoningconfig.go) that fixtures must produce values for

provides:
  - Six NX-OS running-config fixture files covering all parsing categories
  - basic.cfg — device-alias DB, fcalias, all three zone member types, zoneset (PARSE-01/02/03/04)
  - enhanced_mode.cfg — NX-OS 8.5.1+ member device-alias syntax (PARSE-01/03)
  - multi_vsan.cfg — two VSANs with distinct zonesets sharing one device-alias DB (PARSE-06)
  - smart_zoning.cfg — member pwwn with init/target/both keywords (PARSE-03/05)
  - unsupported.cfg — member interface/fcid/ip-address/symbolic-nodename for warn-and-skip (PARSE-05)
  - edge_cases.cfg — IVR zone, empty zone, orphan zone, comment line, device-alias commit

affects:
  - 02-02 (MDS parser implementation — these fixtures are its test inputs)
  - internal/parser/mds/parser_test.go (opened via os.Open filepath.Join)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "NX-OS fixture files use Unix line endings (LF only), no trailing whitespace, no CRLF"
    - "Fixture path convention: testdata/mds/<name>.cfg — opened in tests via filepath.Join"
    - "unsupported.cfg member fcid line includes trailing vsan keyword to exercise full-prefix matching"

key-files:
  created:
    - testdata/mds/basic.cfg
    - testdata/mds/enhanced_mode.cfg
    - testdata/mds/multi_vsan.cfg
    - testdata/mds/smart_zoning.cfg
    - testdata/mds/unsupported.cfg
    - testdata/mds/edge_cases.cfg
  modified: []

key-decisions:
  - "unsupported.cfg includes device-alias database block so parser receives both supported and unsupported members in the same zone, exercising mixed-member handling"
  - "edge_cases.cfg OrphanZone is defined but intentionally absent from any zoneset member list to exercise orphan detection"
  - "smart_zoning.cfg has no device-alias database block — exercises parser path where zone members are raw pWWNs with smart-zoning keywords and no alias resolution needed"

patterns-established:
  - "Fixture spec: exact NX-OS syntax lines drawn from 02-RESEARCH.md Code Examples section (lines 334-444)"
  - "Fixture naming: one file per primary parsing category, not monolithic config"

requirements-completed:
  - PARSE-01
  - PARSE-02
  - PARSE-03
  - PARSE-04
  - PARSE-05
  - PARSE-06

# Metrics
duration: 8min
completed: 2026-03-29
---

# Phase 2 Plan 1: MDS Parser Fixtures Summary

**Six NX-OS running-config fixtures covering all device-alias, fcalias, zone-member, smart-zoning, unsupported-member, multi-VSAN, IVR, and edge-case parsing categories for TDD RED-phase test input**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-29T06:10:24Z
- **Completed:** 2026-03-29T06:17:44Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Created six NX-OS `.cfg` fixtures that encode every distinct syntax form the MDS parser must handle
- Fixtures constitute the TDD RED-phase test input — parser_test.go opens these files directly via filepath.Join
- Covered all six PARSE requirements: device-alias DB (PARSE-01), fcalias (PARSE-02), all member types (PARSE-03), zoneset (PARSE-04), smart-zoning/unsupported (PARSE-05), multi-VSAN (PARSE-06)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create basic.cfg and enhanced_mode.cfg** - `d0a2c6e` (feat)
2. **Task 2: Create multi_vsan.cfg, smart_zoning.cfg, unsupported.cfg, edge_cases.cfg** - `83b0eb7` (feat)

## Files Created/Modified

- `testdata/mds/basic.cfg` — PARSE-01/02/03/04: device-alias DB, fcalias block, zone with device-alias/fcalias/pwwn members, zoneset activation
- `testdata/mds/enhanced_mode.cfg` — PARSE-01/03: NX-OS 8.5.1+ syntax with member device-alias only (no pwwn members)
- `testdata/mds/multi_vsan.cfg` — PARSE-06: two VSANs (10 and 20), each with own zone and zoneset, shared device-alias DB
- `testdata/mds/smart_zoning.cfg` — PARSE-03/05: member pwwn with init/target/both keywords that parser must strip
- `testdata/mds/unsupported.cfg` — PARSE-05: member interface, fcid, ip-address, symbolic-nodename for warn-and-skip path
- `testdata/mds/edge_cases.cfg` — IVR zone (skip+warn), empty zone, orphan zone (not in zoneset), comment line (!), device-alias commit

## Decisions Made

- `unsupported.cfg` includes a device-alias database block with `Server-HBA-A` so the mixed zone exercises both supported (device-alias) and unsupported member types in the same zone block — this matches plan Task 2 spec precisely
- `smart_zoning.cfg` deliberately has no device-alias database block — parser must handle zones where all members are raw pWWNs with trailing smart-zoning keywords
- `edge_cases.cfg` comment line uses `!` prefix (NX-OS comment character) — verifies the parser silently skips unknown lines starting with `!`

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- All six fixtures are ready to be opened by `internal/parser/mds/parser_test.go`
- The exact path pattern is: `os.Open(filepath.Join("..", "..", "..", "testdata", "mds", tt.fixture))`
- Plan 02-02 (MDS parser implementation) can proceed immediately

---
*Phase: 02-mds-parser*
*Completed: 2026-03-29*

## Self-Check: PASSED

All 6 fixture files verified present. Both task commits (d0a2c6e, 83b0eb7) verified in git history.
