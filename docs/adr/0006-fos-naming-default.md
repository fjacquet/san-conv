# ADR-0006: FOS naming default changed to 8.1+ in v1.0

**Date:** 2026-03-29
**Status:** Accepted (supersedes initial pre-8.1 default)

## Context

The `--fos-version` flag controls which character set is accepted in FOS zone/alias names:

- `pre-8.1`: Only `[A-Za-z0-9_]` — hyphens replaced with underscores
- `8.1+`: `[A-Za-z0-9_$^-]` — hyphens, dollar, and caret preserved

At project inception (2026-03-28), the default was set to `pre-8.1` (conservative) to avoid accidental name corruption on older switches. However:

- FOS 8.1 was released in 2017.
- The current FOS release is FOS 10.x (2026).
- Any Brocade switch purchased in the last 8 years runs FOS 8.1 or later.
- The `pre-8.1` default causes unnecessary name mangling on every modern switch, replacing hyphens silently and generating spurious warnings.

## Decision

Change the default value of `--fos-version` from `pre-8.1` to `8.1+` in all CLI entry points (`cmd/root.go`, `cmd/mds2brocade.go`).

Users targeting a legacy switch must explicitly pass `--fos-version pre-8.1`.

## Rationale

- **Modern fleet**: FOS 8.1 is 8 years old. Defaulting to a 2017-era compatibility mode is the wrong trade-off in 2026.
- **Fewer false warnings**: The `pre-8.1` default generates a warning for every hyphenated name — which is the majority of real-world MDS alias names (e.g., `SRV-ESX01-HBA0`). These warnings flood the summary and make genuine warnings harder to spot.
- **Explicit opt-out is safer**: A user running against an old switch will immediately see hyphens preserved in output, which is correct; if it fails, they set the flag. Under the old default, names were silently mangled with no indication that `pre-8.1` mode was active.

## Consequences

- `--fos-version` default changes from `"pre-8.1"` to `"8.1+"` in `cmd/root.go` and `cmd/mds2brocade.go`
- Documentation (README, USER_GUIDE) updated to reflect new default
- Users running against FOS < 8.1 must add `--fos-version pre-8.1` explicitly — this is a minor breaking change but the correct behavior for a v1.0 release
- The `8.1+` regex (`[^A-Za-z0-9_$^-]`) replaces the conservative regex as the runtime default
