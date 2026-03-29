---
phase: 01-foundation
plan: "02"
subsystem: cli
tags: [go, cobra, golangci-lint, goreleaser, cli, build-tooling]

# Dependency graph
requires:
  - phase: 01-foundation/01-01
    provides: Go module with cobra dependency, main.go calling cmd.Execute(), IR structs

provides:
  - Cobra CLI skeleton with mds2brocade and brocade2mds subcommands (--help works on both)
  - mds2brocade stub with --output, --script, --fos-version flags
  - brocade2mds stub with --output flag
  - golangci-lint v2 config passing with zero issues on skeleton
  - goreleaser v2 config validated with CGO_ENABLED=0 and three target platforms

affects:
  - All subsequent phases (every phase builds on CLI skeleton and tooling quality gates)
  - Phase 7 (CLI wiring wires RunE stubs to real pipeline)

# Tech tracking
tech-stack:
  added:
    - golangci-lint v2 (formatters + linters config)
    - goreleaser v2 (cross-platform release config)
  patterns:
    - "Cobra RunE pattern: every command uses RunE (not Run), stubs return fmt.Errorf with 'not yet implemented'"
    - "golangci-lint v2 format: formatters section for gofmt (not linters.enable)"
    - "goreleaser v2 format: archives.formats (plural) not archives.format (singular)"
    - "Static binary: CGO_ENABLED=0 ensures no runtime dependencies for ops team distribution"

key-files:
  created:
    - cmd/root.go
    - cmd/mds2brocade.go
    - cmd/brocade2mds.go
    - .golangci.yml
    - .goreleaser.yml
    - .gitignore
  modified: []

key-decisions:
  - "Use RunE (not Run) on every Cobra command — stubs return non-zero exit on invocation, confirming stub is active and not silently succeeding"
  - "golangci-lint v2: gofmt is a formatter, not a linter — moved to formatters.enable section"
  - "goreleaser v2: archives.formats (plural list) replaces deprecated archives.format (string)"
  - "goreleaser v2: snapshot.version_template replaces deprecated snapshot.name_template"
  - "Git remote added (github.com/fjacquet/san-conv) to satisfy goreleaser check remote validation"

patterns-established:
  - "Pattern 1: Cobra command stubs return fmt.Errorf('cmd: not yet implemented') so invocation exits non-zero"
  - "Pattern 2: golangci-lint v2 config uses formatters: section for formatters (gofmt, goimports) separately from linters:"
  - "Pattern 3: goreleaser v2 archives use formats: (list) not format: (string)"

requirements-completed:
  - CLI-07

# Metrics
duration: 5min
completed: "2026-03-29"
---

# Phase 1 Plan 2: CLI Skeleton and Dev Tooling Summary

**Cobra CLI skeleton with mds2brocade/brocade2mds stubs plus golangci-lint v2 and goreleaser v2 configs — zero lint issues, goreleaser check validated, all help flags visible**

## Performance

- **Duration:** 5 min
- **Started:** 2026-03-29T05:04:00Z
- **Completed:** 2026-03-29T05:08:28Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments

- Both subcommands print complete `--help` output with all flags (`--output`, `--script`, `--fos-version` for mds2brocade; `--output` for brocade2mds)
- Both stubs return exit code 1 when invoked (confirming RunE non-zero behavior)
- golangci-lint v2 config passes with 0 issues on skeleton code
- goreleaser v2 config validated with CGO_ENABLED=0 and linux/darwin/windows targets

## Task Commits

Each task was committed atomically:

1. **Task 1: Create cobra CLI skeleton (cmd/ package)** - `a03ba74` (chore: .gitignore added; cmd/ files were committed in 01-01)
2. **Task 2: Configure golangci-lint v2 and goreleaser v2** - `78acf2b` (chore)

**Plan metadata:** (docs commit pending)

## Files Created/Modified

- `cmd/root.go` - Cobra root command with Execute() and AddCommand for both subcommands
- `cmd/mds2brocade.go` - mds2brocade stub with RunE and --output/--script/--fos-version flags
- `cmd/brocade2mds.go` - brocade2mds stub with RunE and --output flag
- `.golangci.yml` - golangci-lint v2 config: standard preset + misspell linter, gofmt formatter
- `.goreleaser.yml` - goreleaser v2 config: CGO_ENABLED=0, linux/darwin/windows, amd64/arm64
- `.gitignore` - excludes san-conv binary, dist/, .vscode/, .DS_Store

## Decisions Made

- Used RunE (not Run) on all cobra commands so stubs return non-zero and don't silently succeed
- golangci-lint v2 requires `gofmt` in `formatters:` section, not `linters.enable:` — plan spec contained a v2 incompatibility that was auto-corrected
- goreleaser v2 requires `archives.formats:` (list) not `archives.format:` (string) — updated during validation
- Added git remote origin to satisfy goreleaser's remote validation during `check`

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] golangci-lint v2 gofmt formatter config fix**

- **Found during:** Task 2 (golangci-lint validation)
- **Issue:** Plan spec placed `gofmt` in `linters.enable:` but golangci-lint v2 treats gofmt as a formatter (not a linter) — produces fatal error "can't load config: gofmt is a formatter"
- **Fix:** Moved `gofmt` from `linters.enable:` to a new `formatters.enable:` section
- **Files modified:** `.golangci.yml`
- **Verification:** `golangci-lint run ./...` exits 0 with "0 issues"
- **Committed in:** `78acf2b` (Task 2 commit)

**2. [Rule 1 - Bug] goreleaser v2 deprecated archives format fields**

- **Found during:** Task 2 (goreleaser check validation)
- **Issue:** Plan spec used `archives.format: tar.gz` and `format_overrides[].format:` — both deprecated in goreleaser v2.x, causing `check` to fail
- **Fix:** Changed to `archives.formats: [tar.gz]` (list) and `format_overrides[].formats: [zip]`
- **Files modified:** `.goreleaser.yml`
- **Verification:** `goreleaser check` exits 0 with "1 configuration file(s) validated"
- **Committed in:** `78acf2b` (Task 2 commit)

**3. [Rule 1 - Bug] goreleaser v2 deprecated snapshot.name_template**

- **Found during:** Task 2 (goreleaser check validation)
- **Issue:** Plan spec used `snapshot.name_template:` — deprecated in goreleaser v2.x
- **Fix:** Changed to `snapshot.version_template:`
- **Files modified:** `.goreleaser.yml`
- **Verification:** No deprecation warnings in `goreleaser check` output
- **Committed in:** `78acf2b` (Task 2 commit)

**4. [Rule 2 - Missing Critical] Added .gitignore**

- **Found during:** Task 1 (post-build git status)
- **Issue:** Built binary `san-conv` was untracked and would pollute commits; no .gitignore existed
- **Fix:** Created `.gitignore` excluding san-conv binary, dist/, .vscode/, .DS_Store
- **Files modified:** `.gitignore` (created)
- **Verification:** `git status` no longer shows san-conv binary as untracked
- **Committed in:** `a03ba74` (Task 1 commit)

---

**Total deviations:** 4 auto-fixed (3 Rule 1 - bugs in plan spec, 1 Rule 2 - missing .gitignore)
**Impact on plan:** All auto-fixes necessary to achieve acceptance criteria. Plan spec contained goreleaser/golangci-lint v2 API details from docs that were slightly stale — updated to actual installed v2 behavior.

## Issues Encountered

- goreleaser check requires a configured git remote — added `github.com/fjacquet/san-conv.git` as `origin` to enable check validation

## Known Stubs

- `cmd/mds2brocade.go` RunE: returns `fmt.Errorf("mds2brocade: not yet implemented")` — intentional stub, will be wired in Phase 7
- `cmd/brocade2mds.go` RunE: returns `fmt.Errorf("brocade2mds: not yet implemented")` — intentional stub, will be wired in Phase 7

These stubs are intentional by plan design. Phase 7 (CLI Wiring) resolves them.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Phase 1 Foundation complete: compilable binary, --help on both subcommands, zero lint errors, goreleaser config valid
- Phase 2 (MDS Parser) can begin immediately — IR structs and cmd skeleton are the foundation it needs
- Blockers carried from 01-01: multi-VSAN output strategy and test fixture availability remain open concerns for Phase 2

---
*Phase: 01-foundation*
*Completed: 2026-03-29*
