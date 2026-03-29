---
phase: 05-brocade-emitter
plan: 02
subsystem: emitter
tags: [go, brocade, fos, tdd, green-phase, emitter, io.Writer, sorted-keys, script-mode]

# Dependency graph
requires:
  - phase: 05-01-brocade-emitter-red
    provides: 10 table-driven tests defining the complete emitter behavioral contract
  - phase: 04-validator-and-sanitizer
    provides: sanitized *ir.ZoningConfig with FOS-compatible names
  - phase: 01-foundation
    provides: ir package with ZoningConfig, Alias, Zone, ZoneConfig, ZoneMember structs
provides:
  - internal/emitter/brocade/emitter.go with Emit() function producing FOS CLI commands from IR
  - All 10 emitter tests passing (TDD green phase complete)
affects: [06-mds-emitter, 07-cli-wiring]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Generic helper sortedStringKeys[V any](m map[string]V) []string for deterministic map iteration"
    - "emittedZones map[string]bool as a set to filter cfgcreate from skipped zones"
    - "fmt.Fprintf with explicit \\\"...\\\" format strings — NOT %q (avoids Go escape sequences in FOS output)"
    - "zone.Name field used in commands, not map key — avoids @vsanN composite key leakage"
    - "cfg.Warnings mutation: Emit appends warning messages for skipped empty zones"

key-files:
  created:
    - internal/emitter/brocade/emitter.go
  modified: []

key-decisions:
  - "sortedStringKeys uses Go generics (Go 1.18+ type parameters) instead of three separate type-specific helpers — cleaner with no lint warnings"
  - "defzone --noaccess followed by blank line in script mode — test 6 requires HasPrefix 'defzone --noaccess\\n' not HasPrefix 'defzone --noaccess\\n\\n'"
  - "cfgenable comment emitted before cfgsave in postamble — test 7 requires cfgEnablePos < cfgsavePos"
  - "Empty cfgcreate (all zones skipped) is silently dropped — consistent with zone-skip warn-and-continue approach"

patterns-established:
  - "Pattern: generic sortedStringKeys[V any] helper replaces repetitive type-specific sort helpers"
  - "Pattern: emittedZones set-map filters downstream commands from upstream skips"

requirements-completed: [CONV-01, CONV-02, CONV-03, OUT-01, OUT-02]

# Metrics
duration: 4min
completed: 2026-03-29
---

# Phase 5 Plan 02: Brocade FOS Emitter — TDD Green Phase Summary

**Emit() function producing ready-to-paste FOS CLI commands (alicreate/zonecreate/cfgcreate) with sorted map keys, script mode preamble/postamble, and empty zone skip with warnings — all 10 tests passing**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-29T14:13:44Z
- **Completed:** 2026-03-29T14:18:00Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments

- Implemented Emit() function in internal/emitter/brocade/emitter.go (124 lines)
- All 10 table-driven emitter tests pass (TDD green phase complete)
- Correct FOS CLI syntax: alicreate/zonecreate/cfgcreate with explicit double-quotes (not %q)
- Script mode wraps output with defzone --noaccess preamble and cfgsave/commented-cfgenable postamble
- Deterministic output via generic sortedStringKeys helper used for all three map iterations
- Empty zones (all members unsupported) skipped with warning appended to cfg.Warnings
- cfgcreate ZoneNames filtered via emittedZones set to exclude skipped zones
- Full project builds and vets cleanly (go build ./..., go vet ./...)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Emit() function making all emitter tests pass** - `c64d960` (feat)

## Files Created/Modified

- `internal/emitter/brocade/emitter.go` — 124-line Emit() implementation with sortedStringKeys helper; all 10 tests pass

## Decisions Made

- **Generic sortedStringKeys helper:** Used Go 1.18+ type parameters `sortedStringKeys[V any](m map[string]V) []string` instead of three separate type-specific helpers. Cleaner code, no lint warnings, works across Aliases/Zones/ZoneConfigs maps.
- **Script mode blank line after preamble:** `defzone --noaccess` followed immediately by `fmt.Fprintln(w)` emits a blank line separating preamble from command body, matching expected FOS script formatting.
- **cfgenable comment ordering:** Emitted as a for-loop over cfgKeys in the postamble, before `cfgsave` — satisfies Test 7's requirement that cfgEnablePos < cfgsavePos.
- **Empty cfgcreate silently dropped:** When all zones in a cfgcreate are skipped, the cfgcreate is omitted with no warning — consistent with zone-skip warn-and-continue approach; adding a redundant warning for an empty cfgcreate referencing nothing would be noise.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None — Emit() is fully wired to produce real FOS CLI output from IR data.

## Next Phase Readiness

- Brocade emitter complete and tested — ready for Phase 06 (MDS emitter) and Phase 07 (CLI wiring)
- Emit() signature `func Emit(cfg *ir.ZoningConfig, w io.Writer, scriptMode bool) error` is stable for CLI wiring
- io.Writer abstraction makes it trivial to wire to os.Stdout or bytes.Buffer in CLI and tests

## Self-Check: PASSED

- internal/emitter/brocade/emitter.go: FOUND
- .planning/phases/05-brocade-emitter/05-02-SUMMARY.md: FOUND
- Commit c64d960: FOUND

---
*Phase: 05-brocade-emitter*
*Completed: 2026-03-29*
