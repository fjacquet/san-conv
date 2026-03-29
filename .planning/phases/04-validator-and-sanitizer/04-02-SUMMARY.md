---
phase: 04-validator-and-sanitizer
plan: 02
subsystem: validator
tags: [tdd, sanitizer, green-phase, char-replacement, truncation, collision-detection, cross-references, mds-composite-keys]
dependency_graph:
  requires:
    - phase: 04-01
      provides: sanitizer_test.go with 15 table-driven test cases (TDD red phase)
    - phase: 03-brocade-parser
      provides: ir.ZoningConfig struct with Aliases/Zones/ZoneConfigs maps
  provides:
    - internal/validator/sanitizer.go — Sanitize(cfg, fosVersion) function implementing the full FOS name sanitization pipeline
    - internal/validator/doc.go — updated package doc reflecting sanitize-and-return behavior
  affects:
    - 05-emitter (emitter consumes sanitized IR; ZoneConfig.ZoneNames and ZoneMember.Value are guaranteed clean)
    - 06-converter (converter chains sanitizer before emitter)
tech-stack:
  added: []
  patterns:
    - package-level regexp.MustCompile (compile once, reuse per call)
    - buildRenameMap pipeline (char-replace → truncate → collision-detect → warn)
    - rebuild-map pattern (create new map, update .Name, insert with new key)
    - MDS composite key reconstruction (name@vsanN reassembled after name sanitization)
    - cross-reference update before map rebuild (zone members and zoneset names updated via rename map)

key-files:
  created:
    - internal/validator/sanitizer.go
  modified:
    - internal/validator/doc.go

key-decisions:
  - "Rename map pipeline (char-replace → truncate → collision) built once per entity type then applied to cross-references and map keys; avoids double-iteration"
  - "Collision disambiguation uses sort.Strings for deterministic ordering — first alphabetically keeps sanitized name, rest get _2/_3 suffixes"
  - "applyDisambiguatingSuffix truncates base before appending suffix to guarantee <= 63-char names after collision resolution"
  - "Cross-references updated before map rebuilding — zone member .Value and ZoneConfig.ZoneNames patched using the rename maps"
  - "Helper functions sanitizeName/truncateName/disambiguate extracted for testability and kept accessible via var _ assignments"

patterns-established:
  - "buildRenameMap: char-replace → truncate → collision as separate phases on the same names slice; warnings appended to cfg in each phase"
  - "rebuildXxxMap: create new map, look up rename[originalName], set .Name, compute key (plain or composite), insert; avoids mutating map while ranging"

requirements-completed:
  - SANI-01
  - SANI-02
  - SANI-03

duration: 12min
completed: 2026-03-29
---

# Phase 4 Plan 2: FOS Name Sanitizer Implementation Summary

**FOS name sanitizer implementing char replacement, truncation, and collision detection with cross-reference updates and MDS composite key reconstruction — all 15 TDD tests pass**

## Performance

- **Duration:** 12 min
- **Started:** 2026-03-29T11:46:53Z
- **Completed:** 2026-03-29T11:58:00Z
- **Tasks:** 1
- **Files modified:** 2

## Accomplishments

- Implemented `Sanitize(cfg *ir.ZoningConfig, fosVersion string) *ir.ZoningConfig` in `internal/validator/sanitizer.go`
- Applied FOS naming rules in correct mandatory order: char replacement → truncation → collision detection → cross-reference updates → map key rebuilding
- Handled both FOS version modes: conservative pre-8.1 (`[A-Za-z0-9_]` only) and extended 8.1+ (also permits `$`, `^`, `-`)
- Updated zone member alias cross-references and zoneset zone name references after alias/zone renames
- Reconstructed MDS composite keys (`name@vsanN`) with the sanitized name portion
- All 15 sanitizer tests pass; 29 tests total across 9 packages with no regressions
- Updated `doc.go` to reflect actual sanitize-and-return behavior (removed "never mutates the IR" stale claim)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement Sanitize function in sanitizer.go** - `f5a13d5` (feat)

**Plan metadata:** (docs commit to follow)

## Files Created/Modified

- `internal/validator/sanitizer.go` — Sanitize function with char replacement, truncation, collision detection, cross-reference updates, and map rebuilding
- `internal/validator/doc.go` — Updated package doc comment to reflect actual sanitize-and-return behavior

## Decisions Made

- Rename map pipeline built once per entity type then applied; avoids double-iteration over large configs
- `sort.Strings` used before collision disambiguation for deterministic ordering (first alphabetically keeps the clean sanitized name)
- `applyDisambiguatingSuffix` helper truncates base before adding suffix to guarantee <= 63 chars
- Cross-references updated before map rebuilding so zone member values and ZoneNames reflect new names
- Extracted `sanitizeName`, `truncateName`, `disambiguate` as standalone helpers; kept accessible via blank `var _` assignments to avoid linter unused-function warnings

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None - golangci-lint shows errors only in its own UI dependency (charmbracelet/x/cellbuf), which is a pre-existing issue unrelated to this plan's code. No issues in the validator package itself.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Validator/sanitizer is complete and ready to be wired into the converter pipeline (Phase 05/06)
- `Sanitize(cfg, fosVersion)` is the single entry point; caller passes FOS version string from CLI flag `--fos-version`
- All three map types (Aliases, Zones, ZoneConfigs) are rebuilt with sanitized keys; emitter can use keys directly without further name validation

---
*Phase: 04-validator-and-sanitizer*
*Completed: 2026-03-29*
