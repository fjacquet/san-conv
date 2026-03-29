# Product Requirements Document — san-conv

**Version:** 1.0
**Date:** 2026-03-29
**Status:** Released

---

## Problem Statement

SAN fabric migrations from Cisco MDS (NX-OS) to Brocade FOS require manually translating hundreds of zoning objects — device-aliases, zones, and zonesets — from NX-OS syntax to FOS CLI commands. This is error-prone, time-consuming, and blocks migration windows. No off-the-shelf tool handles this translation reliably, and scripting it from scratch for each migration creates inconsistent, unreviewed one-offs.

## Users

**Primary user: SAN/storage ops engineer**

- Works in enterprise data center environments
- Manages Cisco MDS and/or Brocade SAN fabrics
- Comfortable with CLI; not necessarily a developer
- Needs results they can paste directly into a switch or run as a script
- Does not want to install a Python runtime or manage dependencies
- Needs to trust the output — a missed zone member or wrong alias name breaks connectivity silently

**Secondary user: SAN architect / migration planner**

- Needs to review the scope of a migration (object counts, warnings)
- May run the tool in a dry-review pass before the maintenance window

## Core Value

> Given a full MDS running-config file, produce correct, ready-to-apply Brocade FOS CLI commands and a runnable script — with warnings for anything that couldn't be converted cleanly.

## v1 Requirements (all implemented, 2026-03-29)

### Parsing — MDS (NX-OS)

| ID | Requirement | Status |
|----|-------------|--------|
| PARSE-01 | Parse `device-alias database` section → alias → pWWN mappings | ✅ Complete |
| PARSE-02 | Parse `fcalias name X vsan Y` definitions → per-VSAN alias → pWWN | ✅ Complete |
| PARSE-03 | Parse `zone name X vsan Y` blocks including `member device-alias`, `member fcalias`, `member pwwn` | ✅ Complete |
| PARSE-04 | Parse `zoneset name X vsan Y` blocks and zone membership | ✅ Complete |
| PARSE-05 | Detect unsupported member types (interface, fcid, ip-address, symbolic-nodename), emit named warning, skip member | ✅ Complete |
| PARSE-06 | Handle multi-VSAN configs; merge all VSANs into unified Brocade output | ✅ Complete |

### Parsing — Brocade (FOS)

| ID | Requirement | Status |
|----|-------------|--------|
| PARSE-07 | Parse Brocade `cfgshow` output format including backslash-continuation lines | ✅ Complete |
| PARSE-08 | Parse FOS CLI script format (`alicreate`, `zonecreate`, `cfgcreate` commands) | ✅ Complete |
| PARSE-09 | Auto-detect input format (cfgshow vs CLI script) without user flags | ✅ Complete |

### Conversion

| ID | Requirement | Status |
|----|-------------|--------|
| CONV-01 | MDS `device-alias` / `fcalias` → Brocade `alicreate` | ✅ Complete |
| CONV-02 | MDS `zone` → Brocade `zonecreate`, resolving alias/device-alias references | ✅ Complete |
| CONV-03 | MDS `zoneset` → Brocade `cfgcreate` | ✅ Complete |
| CONV-04 | Brocade `alias:` → MDS `device-alias` | ✅ Complete |
| CONV-05 | Brocade `zone:` → MDS `zone name X vsan 1` | ✅ Complete |
| CONV-06 | Brocade `cfg:` → MDS `zoneset name X vsan 1` | ✅ Complete |

### Name Sanitization

| ID | Requirement | Status |
|----|-------------|--------|
| SANI-01 | Enforce FOS 63-character name limit; truncate with warning | ✅ Complete |
| SANI-02 | Replace invalid characters for target FOS version; warn per name | ✅ Complete |
| SANI-03 | Detect post-sanitization collisions; disambiguate with `_2`/`_3` suffixes; collision warning | ✅ Complete |

### Output

| ID | Requirement | Status |
|----|-------------|--------|
| OUT-01 | Write FOS CLI commands in application order: `alicreate` → `zonecreate` → `cfgcreate` | ✅ Complete |
| OUT-02 | Generate executable shell script with `defzone --noaccess`, zone commands, `cfgenable`, `cfgsave` | ✅ Complete |
| OUT-03 | Write NX-OS config commands for Brocade→MDS direction | ✅ Complete |
| OUT-04 | Print conversion summary to stderr: object counts and warning count | ✅ Complete |

### CLI Interface

| ID | Requirement | Status |
|----|-------------|--------|
| CLI-01 | Accept input file as positional argument | ✅ Complete |
| CLI-02 | `--direction` flag: `mds2brocade` (default) or `brocade2mds` | ✅ Complete |
| CLI-03 | `--output` flag: write primary output to file instead of stdout | ✅ Complete |
| CLI-04 | `--script` flag: also write executable shell script | ✅ Complete |
| CLI-05 | `--fos-version` flag: `pre-8.1` (default) or `8.1+` | ✅ Complete |
| CLI-06 | Exit 0 on success (warnings allowed); non-zero on fatal IO/parse errors | ✅ Complete |
| CLI-07 | Single distributable Go binary; no runtime dependencies | ✅ Complete |

## v2 Backlog

| ID | Requirement | Priority |
|----|-------------|----------|
| VAL-01 | Roundtrip validation mode: convert both directions and diff | Medium |
| VAL-02 | Validate output against target platform naming rules before writing | Medium |
| VAL-03 | `--dry-run` flag: validate without writing output | Low |
| ADV-01 | Accept Brocade `configupload` full backup format | Medium |
| ADV-02 | `--vsan` flag: convert a single named VSAN | Low |
| INT-01 | Read from stdin (piped config data) | Low |
| INT-02 | Structured JSON output for machine-readable consumption | Low |

## Out of Scope

| Feature | Reason |
|---------|--------|
| SSH / live switch connection | Tool is offline static analysis only; ops team applies output manually |
| GUI or web interface | CLI only; ops team uses CLI workflows |
| VSAN topology mapping | Requires network knowledge beyond the config file |
| Zone enforcement mode (hard/soft) | FOS and NX-OS semantics differ in ways that can't be auto-translated |
| IVR zones | Inter-VSAN Routing has no FOS equivalent; skip with warning |
| Smart zoning keywords (init/target/both) | No FOS equivalent; strip keyword, preserve pWWN with warning |
| TI (Traffic Isolation) zones | Deprecated in modern NX-OS; skip with warning |

## Constraints

| Constraint | Detail |
|-----------|--------|
| Tech stack | Go — single binary, no runtime deps |
| Error handling | Warn and continue — partial output is better than stopping mid-conversion |
| Input | Full config file, not live switch connection |
| Compatibility | Must handle real-world MDS configs: empty zones, comments, long names, enhanced device-alias mode |

## Success Metrics (v1)

- A 500-alias, 300-zone MDS config converts to valid FOS commands in < 1 second
- Zero silent data loss — every skipped member has a warning
- `go test ./... -race` passes on linux/darwin/windows
- Ops engineer can install with `go install github.com/fjacquet/san-conv@latest` in one command

---
*PRD owner: fjacquet — Last updated 2026-03-29*
