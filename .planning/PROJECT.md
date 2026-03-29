# san-conv — SAN Zoning Config Converter

## What This Is

A Go CLI tool that converts SAN fabric zoning configurations between Cisco MDS (NX-OS) and Brocade FOS formats. The primary use case is migrating zoning from Cisco MDS to Brocade switches, with bidirectional conversion supported. It is built for ops teams who need a reliable, distributable binary with no runtime dependencies.

## Core Value

Given a full MDS running-config file, produce correct, ready-to-apply Brocade FOS CLI commands and a runnable script — with warnings for anything that couldn't be converted cleanly.

## Requirements

### Validated

- [x] Parse full Cisco MDS running-config files to extract zoning objects — Validated in Phase 02: mds-parser
- [x] Parse Brocade FOS config files to extract zoning objects — Validated in Phase 03: brocade-parser
- [x] Warn on unconvertible/ambiguous constructs and continue (best-effort) — Validated in Phase 04: validator-and-sanitizer (name sanitization warnings)
- [x] Convert device-alias (MDS) ↔ alias (Brocade) — Validated in Phase 05: brocade-emitter (alicreate emission)
- [x] Convert zone definitions including pWWN/alias members — Validated in Phase 05: brocade-emitter (zonecreate emission)
- [x] Convert zoneset (MDS) ↔ cfg (Brocade) — Validated in Phase 05: brocade-emitter (cfgcreate emission)
- [x] Output Brocade FOS CLI commands (ready-to-paste) — Validated in Phase 05: brocade-emitter
- [x] Output NX-OS CLI commands for brocade2mds direction — Validated in Phase 06: mds-emitter
- [x] CLI flag `--direction mds2brocade|brocade2mds` — Validated in Phase 07: cli-wiring-and-integration
- [x] `--output`, `--script`, `--fos-version` flags operational — Validated in Phase 07: cli-wiring-and-integration
- [x] Flat invocation `san-conv myconfig.txt` works end-to-end — Validated in Phase 07: cli-wiring-and-integration
- [x] stderr summary with object counts and warnings — Validated in Phase 07: cli-wiring-and-integration
- [x] Exit 0 on warnings-only; non-zero on IO errors — Validated in Phase 07: cli-wiring-and-integration

### Active

- [ ] Parse full Cisco MDS running-config files to extract zoning objects
- [ ] Parse Brocade FOS config files to extract zoning objects
- [ ] Convert device-alias (MDS) ↔ alias (Brocade)
- [ ] Convert zone definitions including pWWN/alias members
- [ ] Convert zoneset (MDS) ↔ cfg (Brocade)
- [ ] Output Brocade FOS CLI commands (ready-to-paste)
- [ ] Output executable shell script for FOS commands
- [ ] Warn on unconvertible/ambiguous constructs and continue (best-effort)
- [ ] CLI flag to select conversion direction (mds2brocade / brocade2mds)
- [ ] Single distributable Go binary for ops team use

### Out of Scope

- VSAN-to-fabric mapping — complex topology dependency, requires network knowledge beyond config files
- VSANs without active zoneset — no zoning to convert
- Zone enforcement mode differences (hard/soft) — FOS and NX-OS semantics differ, out of v1 scope
- SSH/remote execution — tool generates commands, ops team applies them; automation deferred
- GUI or web interface — CLI only for v1

## Context

- Cisco MDS uses NX-OS syntax: `device-alias`, `zone name`, `zoneset name`, `zoneset activate`
- Brocade FOS uses: `alicreate`, `zonecreate`, `cfgcreate`, `cfgenable`
- Input is a full `show running-config` export from MDS, or equivalent Brocade config dump
- Both formats use WWNs (pWWN) as zone members; alias/device-alias names may differ in convention
- Conversion warnings needed for: name length mismatches, special characters in names, features with no equivalent
- Ops team needs an easy install — single binary via `go install` or released binary download

## Constraints

- **Tech stack**: Go — single binary, no runtime deps, easy to distribute to ops team
- **Error handling**: Warn and continue — partial output is better than stopping mid-conversion
- **Input**: Full config file (not live switch connection) — tool is offline/static analysis only
- **Compatibility**: Must handle real-world MDS configs including edge cases (empty zones, comments, long names)

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| Go over Python | Single binary distribution for ops team, no Python install required | — Pending |
| Warn and continue on errors | Ops teams need best-effort output to review, not a hard stop | — Pending |
| Bidirectional (MDS primary) | MDS→Brocade is the real driver; Brocade→MDS added for completeness | — Pending |
| Output both FOS commands and script | Ops team may paste interactively or run automated; both formats needed | — Pending |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition** (via `/gsd:transition`):

1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

**After each milestone** (via `/gsd:complete-milestone`):

1. Full review of all sections
2. Core Value check — still the right priority?
3. Audit Out of Scope — reasons still valid?
4. Update Context with current state

---
*Last updated: 2026-03-29 — Phase 07 complete: CLI wired end-to-end, v1.0 milestone complete*
