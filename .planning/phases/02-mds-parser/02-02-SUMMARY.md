---
phase: 02-mds-parser
plan: "02"
subsystem: parser
tags: [go, bufio, regexp, mds, nxos, state-machine, tdd]

# Dependency graph
requires:
  - phase: 02-mds-parser/02-01
    provides: Six NX-OS fixture files (testdata/mds/*.cfg) that tests open and parse
  - phase: 01-foundation/01-01
    provides: internal/ir/zoningconfig.go IR structs (ZoningConfig, Alias, Zone, ZoneMember, ZoneConfig)

provides:
  - Two-pass MDS NX-OS running-config parser (internal/parser/mds/parser.go)
  - Table-driven test suite for all six fixture files (internal/parser/mds/parser_test.go)
  - Parse(io.Reader) -> (*ir.ZoningConfig, error) public interface
  - normalizeWWN colon-hex WWN normalization helper
  - isTopLevelKeyword block state transition helper

affects: [03-brocade-parser, 04-validator, 05-emitter, 06-cli]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - Two-pass state machine parser (pass1 aliases, pass2 zones) with package-level compiled regexes
    - Composite zone map key "name@vsanN" for multi-VSAN disambiguation in ir.ZoningConfig.Zones
    - IVR-before-zone regex ordering to prevent IVR lines being mis-parsed as regular zone headers
    - Unsupported member types skipped with warnings; NOT added to zone.Members
    - Smart-zoning role keywords stripped from pWWN members with per-occurrence warnings

key-files:
  created:
    - internal/parser/mds/parser.go
    - internal/parser/mds/parser_test.go
  modified: []

key-decisions:
  - "Two-pass parse (pass1=aliases, pass2=zones): pass1 runs first so fcalias/device-alias data is available for zone member lookups if needed in future phases"
  - "Composite zone map key 'name@vsanN' used for ir.ZoningConfig.Zones and ir.ZoneConfigs to support multi-VSAN fixtures without collision"
  - "IVR regex checks ordered BEFORE zone checks since IVR lines contain 'zone name' substring — wrong order produces silent mis-parse"
  - "wantAliases in basic mode test corrected from 2 to 3: basic.cfg has 2 device-aliases (Server-HBA-A, Storage-Port-1) + 1 fcalias (Server-port-A) = 3 distinct map keys"

patterns-established:
  - "State machine: local const iota states inside each pass function — no package-level state variables"
  - "processZoneMember: unsupported patterns checked FIRST (interface, fcid, ip-address, symbolic-nodename, fwwn) before device-alias/fcalias/pwwn"
  - "normalizeWWN: remove colons, lowercase, verify 16-char compact, reinsert colons every 2 chars"
  - "isTopLevelKeyword: blank lines and '!' comments return false (continue current block); any non-indented non-empty non-comment line returns true"

requirements-completed: [PARSE-01, PARSE-02, PARSE-03, PARSE-04, PARSE-05, PARSE-06]

# Metrics
duration: 4min
completed: 2026-03-29
---

# Phase 02 Plan 02: MDS NX-OS Parser Summary

**Two-pass bufio.Scanner state machine parser producing ir.ZoningConfig from Cisco NX-OS running-config with device-alias/fcalias/zone/zoneset/IVR/smart-zoning support**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-29T06:24:03Z
- **Completed:** 2026-03-29T06:28:00Z
- **Tasks:** 2 (TDD RED + GREEN)
- **Files modified:** 2 (parser.go created, parser_test.go created+fixed)

## Accomplishments

- Implemented full two-pass MDS NX-OS running-config parser in 349 lines with 21 compiled regexes
- All six fixture test cases pass (basic, enhanced_mode, multi_vsan, smart_zoning, unsupported, edge_cases)
- Parser correctly handles device-alias database blocks, fcalias VSAN-scoped aliases, zone/zoneset blocks, IVR zone skipping, smart-zoning role stripping, and 5 unsupported member types

## Task Commits

Each task was committed atomically:

1. **Task 1: Write parser_test.go with six failing test cases (RED phase)** - `4f706a0` (test)
2. **Task 2: Implement parser.go two-pass state machine (GREEN phase)** - `fb645ae` (feat)

## Files Created/Modified

- `/Users/fjacquet/Projects/san-conv/internal/parser/mds/parser.go` — Two-pass state machine parser, 349 lines; exports Parse(io.Reader) (*ir.ZoningConfig, error)
- `/Users/fjacquet/Projects/san-conv/internal/parser/mds/parser_test.go` — Six table-driven fixture tests, 175 lines; all pass

## Decisions Made

- Two-pass design separates alias resolution (pass1) from zone/zoneset processing (pass2), keeping each pass focused
- IVR regex check ordered before zone check: IVR lines contain "zone name" substring — wrong order silently mis-parses IVR as regular zones
- Composite key "name@vsanN" for both cfg.Zones and cfg.ZoneConfigs maps enables multi-VSAN support without collision
- Unsupported member types (interface, fcid, ip-address, symbolic-nodename, fwwn) are skipped entirely (not added to Members) with warning appended — matches plan spec PARSE-05

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Corrected wantAliases from 2 to 3 in basic mode test**

- **Found during:** Task 2 (GREEN phase — first test run)
- **Issue:** Plan spec stated `wantAliases=2` for basic.cfg, but basic.cfg contains 2 device-aliases (Server-HBA-A, Storage-Port-1) + 1 fcalias (Server-port-A) = 3 distinct Aliases map keys. Test failed with "has 3 items, expected 2"
- **Fix:** Updated test assertion from `Len(t, cfg.Aliases, 2, ...)` to `Len(t, cfg.Aliases, 3, ...)` with accurate comment explaining the 2+1 breakdown
- **Files modified:** internal/parser/mds/parser_test.go (line 26)
- **Verification:** All 6 fixture tests pass after fix
- **Committed in:** fb645ae (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (Rule 1 - bug in plan's expected count, fixture is ground truth)
**Impact on plan:** Single assertion count fix. Parser behavior is correct; the plan's wantAliases=2 was a miscounting of the fixture file. No scope creep.

## Issues Encountered

None beyond the alias count deviation documented above.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- MDS parser complete; `Parse(io.Reader)` interface ready for CLI wiring in Phase 6
- `ir.ZoningConfig` populated correctly by parser; ready for Brocade emitter (Phase 5) consumption
- Multi-VSAN handling produces composite keys — downstream phases need awareness of "name@vsanN" key convention
- Blocker from STATE.md (multi-VSAN output strategy) is documented but does not block Phase 3 (Brocade parser)

---
*Phase: 02-mds-parser*
*Completed: 2026-03-29*
