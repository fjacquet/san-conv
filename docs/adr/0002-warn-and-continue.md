# ADR-0002: Warn and continue on non-fatal errors

**Date:** 2026-03-28
**Status:** Accepted

## Context

Real-world MDS configs contain constructs that have no FOS equivalent: IVR zones, TI zones, interface members, smart-zoning keywords (`init`/`target`/`both`). A conversion tool that stops on the first such construct produces no usable output, forcing the ops engineer to clean the input file before every run.

A migration window has a fixed duration. The ops team needs best-effort output they can review and augment manually — not a hard stop that leaves them with nothing.

## Decision

The tool never exits non-zero due to a skipped construct or name sanitization. Every non-fatal issue appends a human-readable warning to `cfg.Warnings` (which is flushed to stderr). The tool exits 1 only on IO errors (file not found, write failure) and hard parse failures (malformed input with no recoverable content).

## Rationale

- **Partial output beats no output**: A conversion with 3 skipped IVR zones is still 95% of the work done. The engineer reviews the warning list and handles exceptions manually.
- **Warnings are non-lossy**: Every skipped member is named in the warning. The engineer has a complete record of what was dropped.
- **Ops workflow alignment**: Ops teams are accustomed to reviewing diffs and change logs. A warning list is natural; an abort is not.

## Consequences

- The summary line `Summary: N aliases, N zones, N configs | Warnings: N` always appears on stderr, even for zero-warning runs.
- Exit code 0 with warnings is NOT an error condition; callers must read stderr to detect warnings.
- The `cfg.Warnings []string` slice is the canonical warning channel — all packages append to it rather than writing directly to stderr.
- Future `--strict` flag (v2) could flip the behavior to exit 1 on any warning, for automated validation pipelines.
