---
phase: 04-validator-and-sanitizer
plan: 01
subsystem: validator
tags: [tdd, sanitizer, red-phase, testing]
dependency_graph:
  requires: [internal/ir/zoningconfig.go, internal/validator/doc.go]
  provides: [internal/validator/sanitizer_test.go]
  affects: [internal/validator/sanitizer.go (to be created in 04-02)]
tech_stack:
  added: []
  patterns: [table-driven-tests, tdd-red-phase, inline-ir-construction, require-assertions]
key_files:
  created: [internal/validator/sanitizer_test.go]
  modified: []
decisions:
  - "15 table-driven test cases cover all SANI-01/02/03 requirements plus cross-reference updates and MDS composite key handling"
  - "Helper functions (makeAlias, makeZone, makeZoneVSAN, makeZoneConfig, makeCfg, makeMDSCfg) reduce test boilerplate while keeping all IR inline"
  - "Sanitize() function signature: func Sanitize(cfg *ir.ZoningConfig, fosVersion string) *ir.ZoningConfig"
metrics:
  duration: "1m 46s"
  completed: "2026-03-29"
  tasks: 1
  files: 1
---

# Phase 4 Plan 1: FOS Name Sanitizer TDD Red Phase Summary

Table-driven test suite for the FOS name sanitizer with 15 cases covering SANI-01 (truncation), SANI-02 (char replacement per FOS version), SANI-03 (collision detection), cross-reference updates, and MDS composite key reconstruction. Tests fail with `undefined: Sanitize` as expected (TDD red phase).

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create sanitizer_test.go with complete table-driven test suite | 22d2cc9 | internal/validator/sanitizer_test.go |

## Verification Results

- `go vet ./internal/validator/...` returns "undefined: Sanitize" (1 error — expected, TDD red phase)
- 15 test cases in `TestSanitize` (exceeds minimum of 14)
- 44 `require.` assertions (all use `require`, not `assert`, per CLAUDE.md)
- `ir.ZoningConfig` constructed inline in 38 locations (no fixture files)
- File is in `package validator` matching doc.go

## Test Coverage

| Requirement | Test Cases |
|-------------|------------|
| SANI-01 (truncation) | alias exceeding 63 chars, zone exceeding 63 chars, name exactly 63 chars |
| SANI-02 (char replacement) | hyphen pre-8.1, dollar/caret pre-8.1, hyphen 8.1+, dollar/caret 8.1+, at-sign both modes |
| SANI-03 (collision detection) | alias collision, zone collision, suffix length guard |
| Cross-reference updates | zone member alias value, zoneset zone name |
| MDS composite key | composite key reconstruction with sanitized name |
| No-op | clean names produce no warnings |

## Decisions Made

- Sanitize() function takes `(cfg *ir.ZoningConfig, fosVersion string) *ir.ZoningConfig` — matches research recommendation
- Helper functions kept within the test file to reduce boilerplate without introducing test fixtures
- makeMDSCfg() helper added for MDS-specific tests that require composite key behavior
- Test names follow the description pattern from the plan exactly to satisfy plan acceptance criteria grep matches

## Deviations from Plan

None — plan executed exactly as written.

## Self-Check: PASSED

- FOUND: internal/validator/sanitizer_test.go
- FOUND: commit 22d2cc9
