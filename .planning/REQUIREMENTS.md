# Requirements: san-conv

**Defined:** 2026-03-28
**Core Value:** Given a full MDS running-config file, produce correct, ready-to-apply Brocade FOS CLI commands and a runnable script — with warnings for anything that couldn't be converted cleanly.

## v1 Requirements

### Parsing — MDS (NX-OS)

- [x] **PARSE-01**: Tool parses `device-alias database` section to extract fabric-wide alias → pWWN mappings
- [x] **PARSE-02**: Tool parses `fcalias name X vsan Y` definitions to extract per-VSAN alias → pWWN mappings
- [x] **PARSE-03**: Tool parses `zone name X vsan Y` blocks including `member device-alias`, `member fcalias`, and `member pwwn` entries
- [x] **PARSE-04**: Tool parses `zoneset name X vsan Y` blocks and their zone membership
- [x] **PARSE-05**: Tool detects unsupported member types (interface, fcid, ip-address, symbolic-nodename) and emits a named warning per occurrence, then skips the member
- [x] **PARSE-06**: Tool handles multi-VSAN configs and converts all VSANs into merged Brocade fabric output

### Parsing — Brocade (FOS)

- [x] **PARSE-07**: Tool parses Brocade `cfgshow` output format (`Defined configuration:` section with `alias:`, `zone:`, `cfg:` lines including backslash-continuation)
- [x] **PARSE-08**: Tool parses Brocade FOS CLI script format (`alicreate`, `zonecreate`, `cfgcreate` commands)
- [x] **PARSE-09**: Tool auto-detects whether Brocade input is cfgshow output or CLI script format

### Conversion

- [ ] **CONV-01**: Tool converts MDS `device-alias` and `fcalias` entries to Brocade `alicreate` commands with pWWN members
- [ ] **CONV-02**: Tool converts MDS `zone` definitions to Brocade `zonecreate` commands, resolving alias/device-alias references
- [ ] **CONV-03**: Tool converts MDS `zoneset` definitions to Brocade `cfgcreate` commands
- [ ] **CONV-04**: Tool converts Brocade `alias:` definitions to MDS `device-alias` entries
- [ ] **CONV-05**: Tool converts Brocade `zone:` definitions to MDS `zone name X vsan 1` definitions
- [ ] **CONV-06**: Tool converts Brocade `cfg:` definitions to MDS `zoneset name X vsan 1` definitions

### Name Sanitization

- [ ] **SANI-01**: Tool enforces FOS 63-character name limit and truncates names that exceed it, emitting a warning with old and new names
- [ ] **SANI-02**: Tool replaces characters invalid in conservative FOS naming (only `[A-Za-z0-9_$^-]` allowed in FOS 8.1+; `[A-Za-z0-9_]` in default mode) and warns on each replacement
- [ ] **SANI-03**: Tool detects when two or more names become identical after sanitization and emits a collision warning with all affected names

### Output

- [ ] **OUT-01**: Tool writes Brocade FOS CLI commands to stdout (or `--output` file) in correct application order: alicreate → zonecreate → cfgcreate
- [ ] **OUT-02**: Tool generates an executable shell script that includes `defzone --noaccess` preamble, all zone commands, `cfgenable <cfg-name>`, and `cfgsave` postamble
- [ ] **OUT-03**: Tool writes MDS NX-OS config commands for the Brocade→MDS direction (device-alias, zone, zoneset, zoneset activate)
- [ ] **OUT-04**: Tool prints a conversion summary to stderr listing: objects converted, objects skipped, warnings issued

### CLI Interface

- [ ] **CLI-01**: Tool accepts the input config file path as a positional argument
- [ ] **CLI-02**: Tool provides `--direction` flag with values `mds2brocade` (default) and `brocade2mds`
- [ ] **CLI-03**: Tool provides `--output` flag to write primary output to a file instead of stdout
- [ ] **CLI-04**: Tool provides `--script` flag to also write a shell script file alongside the primary output
- [ ] **CLI-05**: Tool provides `--fos-version` flag (values: `pre-8.1`, `8.1+`) defaulting to `pre-8.1` (conservative charset)
- [ ] **CLI-06**: Tool exits with code 0 on success (warnings allowed), non-zero only on fatal parse/IO errors
- [x] **CLI-07**: Single distributable Go binary with no runtime dependencies (go install or pre-built release)

## v2 Requirements

### Validation & Roundtrip

- **VAL-01**: Tool supports roundtrip validation mode that converts both directions and diffs the result
- **VAL-02**: Tool validates output against target platform naming rules before writing
- **VAL-03**: Tool provides `--dry-run` flag that validates without writing output

### Advanced Input

- **ADV-01**: Tool accepts Brocade `configupload` full backup file as input (differs from cfgshow format)
- **ADV-02**: Tool provides `--vsan` flag to convert a single named VSAN rather than all

### Integration

- **INT-01**: Tool supports reading from stdin (piped config data)
- **INT-02**: Tool outputs structured JSON for machine-readable consumption

## Out of Scope

| Feature | Reason |
|---------|--------|
| SSH / live switch connection | Tool is offline static analysis only; ops team applies output manually |
| GUI or web interface | CLI only for v1; ops team already uses CLI workflows |
| VSAN topology mapping | Requires network knowledge beyond the config file |
| Zone enforcement mode (hard/soft) | FOS and NX-OS semantics differ in ways that can't be auto-translated |
| IVR zones | Inter-VSAN Routing has no direct FOS equivalent; skip with warning |
| Smart zoning keywords (init/target/both) | No FOS equivalent; strip keyword, preserve member pWWN with warning |
| TI (Traffic Isolation) zones | Deprecated in modern NX-OS; skip with warning |
| cfgenable at parse time | Tool never connects to live switch; cfgenable is in generated script only |

## Traceability

| Requirement | Phase | Status |
|-------------|-------|--------|
| PARSE-01 | Phase 2 | Complete |
| PARSE-02 | Phase 2 | Complete |
| PARSE-03 | Phase 2 | Complete |
| PARSE-04 | Phase 2 | Complete |
| PARSE-05 | Phase 2 | Complete |
| PARSE-06 | Phase 2 | Complete |
| PARSE-07 | Phase 3 | Complete |
| PARSE-08 | Phase 3 | Complete |
| PARSE-09 | Phase 3 | Complete |
| CONV-01 | Phase 5 | Pending |
| CONV-02 | Phase 5 | Pending |
| CONV-03 | Phase 5 | Pending |
| CONV-04 | Phase 6 | Pending |
| CONV-05 | Phase 6 | Pending |
| CONV-06 | Phase 6 | Pending |
| SANI-01 | Phase 4 | Pending |
| SANI-02 | Phase 4 | Pending |
| SANI-03 | Phase 4 | Pending |
| OUT-01 | Phase 5 | Pending |
| OUT-02 | Phase 5 | Pending |
| OUT-03 | Phase 6 | Pending |
| OUT-04 | Phase 7 | Pending |
| CLI-01 | Phase 7 | Pending |
| CLI-02 | Phase 7 | Pending |
| CLI-03 | Phase 7 | Pending |
| CLI-04 | Phase 7 | Pending |
| CLI-05 | Phase 7 | Pending |
| CLI-06 | Phase 7 | Pending |
| CLI-07 | Phase 1 | Complete |

**Coverage:**
- v1 requirements: 29 total
- Mapped to phases: 29
- Unmapped: 0 ✓

---
*Requirements defined: 2026-03-28*
*Last updated: 2026-03-28 after roadmap creation*
