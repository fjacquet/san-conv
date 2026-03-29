---
phase: 07-cli-wiring-and-integration
plan: 03
subsystem: cli
tags: [cobra, go, cli, gap-closure, flags]

# Dependency graph
requires:
  - phase: 07-02-cli-wiring-and-integration
    provides: converter.Run() pipeline orchestrator and cobra command wiring

provides:
  - Root-level positional argument handler on rootCmd (cobra.ExactArgs(1))
  - --direction flag (default: mds2brocade) on rootCmd for flat invocation
  - --output, --script, --fos-version flags on rootCmd for flat invocation
  - Flat invocation pattern: san-conv myconfig.txt (no subcommand) fully wired
  - Bidirectional flat invocation: san-conv myconfig.txt --direction brocade2mds

affects: [release, distribution, user-facing CLI docs]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "cobra subcommand disambiguation: rootCmd.RunE only fires when no recognized subcommand is the first arg"
    - "Flat invocation pattern: rootCmd with ExactArgs(1) + named subcommand aliases coexist safely"

key-files:
  created: []
  modified:
    - cmd/root.go

key-decisions:
  - "cobra routes subcommands before rootCmd.RunE fires — adding Args and RunE to rootCmd does not affect mds2brocade/brocade2mds subcommand routing"
  - "Root-level flags (--direction, --output, --script, --fos-version) declared only on rootCmd, not inherited by subcommands — avoids flag duplication"
  - "SilenceUsage=true kept on rootCmd so ops team sees only error message, not usage block, on file-not-found errors"

patterns-established:
  - "Flat invocation as primary UX, subcommands as named aliases: both work simultaneously in cobra"

requirements-completed: [CLI-01, CLI-02]

# Metrics
duration: 15min
completed: 2026-03-29
---

# Phase 7 Plan 03: Gap Closure (SC#1 + CLI-02) Summary

**Root-level flat invocation wired to converter.Run() with --direction flag, closing SC#1 and CLI-02 gaps while preserving all 61 existing tests**

## Performance

- **Duration:** 15 min
- **Started:** 2026-03-29T15:52:37Z
- **Completed:** 2026-03-29
- **Tasks:** 2 (1 auto + 1 human-verify checkpoint)
- **Files modified:** 1

## Accomplishments
- Added `Args: cobra.ExactArgs(1)` and `RunE` to rootCmd so `san-conv myconfig.txt` runs the default mds2brocade pipeline (closes SC#1)
- Declared `--direction` flag (default: `mds2brocade`) enabling `san-conv myconfig.txt --direction brocade2mds` (closes SC#4 / CLI-02)
- Added companion root flags `--output`, `--script`, `--fos-version` for full parity with subcommand flags
- Human smoke-test verified all four scenarios: flat MDS, flat Brocade, subcommand alias, missing-file exit-1
- All 61 tests continue to pass with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add root-level positional arg handler and --direction flag to rootCmd** - `0c7845a` (feat)
2. **Task 2: Smoke-test flat invocation and --direction flag end-to-end** - human-verify checkpoint (approved; no code change needed)

## Files Created/Modified
- `cmd/root.go` - Added `Args: cobra.ExactArgs(1)`, `RunE` delegating to `converter.Run()`, and four root-level flags (`--direction`, `--output`, `--script`, `--fos-version`)

## Decisions Made
- cobra resolves named subcommands before rootCmd.RunE, so adding RunE to rootCmd is safe and does not interfere with existing `mds2brocade` / `brocade2mds` subcommand routing
- Root flags are declared with `rootCmd.Flags()` (local, not persistent) so they are visible only when rootCmd handles the invocation — avoids leaking duplicate flags into subcommand help text

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None - the cobra subcommand routing guarantee meant no conflict between root RunE and existing subcommands.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - cmd/root.go is fully wired; all flags pass through to converter.Run().

## Next Phase Readiness
- Phase 7 is now complete: all three plans executed, all CLI requirements satisfied
- SC#1 (flat invocation), SC#4 (--direction flag), and CLI-02 are closed
- The binary is ready for goreleaser cross-platform release (Phase 7 SC#6 via goreleaser config from Phase 1)
- No blockers for distribution

## Self-Check

- `cmd/root.go` present and contains `cobra.ExactArgs`, `converter.Run`, `direction` flag: confirmed via Task 1 commit 0c7845a
- `go build ./...` and `go test ./...` (61/61) confirmed by human approval at checkpoint Task 2
- SUMMARY.md written to `.planning/phases/07-cli-wiring-and-integration/07-03-SUMMARY.md`

## Self-Check: PASSED

---
*Phase: 07-cli-wiring-and-integration*
*Completed: 2026-03-29*
