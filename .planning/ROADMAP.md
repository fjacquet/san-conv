# Roadmap: san-conv

## Overview

san-conv is built as a strict compiler pipeline: IR definition first, then parsers producing it, then validator sanitizing it, then emitters consuming it, and finally CLI wiring the whole pipeline together. Each phase delivers a complete, independently testable component. The MDS→Brocade direction is the primary use case and drives phase ordering: MDS parser (Phase 2) and Brocade emitter (Phase 5) are the critical path. The Brocade parser (Phase 3) and MDS emitter (Phase 6) are the symmetric counterparts, built after their direction's primary component proves the IR contract.

## Phases

**Phase Numbering:**
- Integer phases (1, 2, 3): Planned milestone work
- Decimal phases (2.1, 2.2): Urgent insertions (marked with INSERTED)

Decimal phases appear between their surrounding integers in numeric order.

- [x] **Phase 1: Foundation** - IR struct definitions, project scaffolding, and compilable binary skeleton (completed 2026-03-29)
- [ ] **Phase 2: MDS Parser** - Full NX-OS running-config parsing producing validated IR
- [ ] **Phase 3: Brocade Parser** - FOS cfgshow and CLI script parsing producing validated IR
- [ ] **Phase 4: Validator and Sanitizer** - Name sanitization and post-sanitization collision detection
- [ ] **Phase 5: Brocade Emitter** - FOS CLI command generation from IR (primary output path)
- [ ] **Phase 6: MDS Emitter** - NX-OS CLI command generation from IR (reverse direction)
- [ ] **Phase 7: CLI Wiring and Integration** - Complete pipeline wiring, flags, summary output, and release binary

## Phase Details

### Phase 1: Foundation
**Goal**: A compilable san-conv binary exists with the complete IR contract and both subcommands stubbed, unblocking all parallel parser and emitter work
**Depends on**: Nothing (first phase)
**Requirements**: CLI-07
**Success Criteria** (what must be TRUE):
  1. `go build ./...` produces a single `san-conv` binary with no errors
  2. `san-conv mds2brocade --help` and `san-conv brocade2mds --help` both print flag help without panicking
  3. `internal/ir/zoningconfig.go` defines `ZoningConfig`, `Alias`, `Zone`, `ZoneMember`, and `ZoneConfig` structs that compile cleanly
  4. `go test ./...` runs (zero tests pass, zero tests fail — no panics)
  5. golangci-lint and goreleaser configs are present and lint passes on the empty skeleton
**Plans**: 2 plans

Plans:
- [x] 01-01-PLAN.md — Go module init, IR struct definitions, and empty internal sub-package stubs
- [x] 01-02-PLAN.md — Cobra CLI skeleton (both subcommands stubbed), golangci-lint v2 config, goreleaser v2 config

### Phase 2: MDS Parser
**Goal**: The MDS parser correctly reads any real NX-OS running-config and produces a fully populated IR struct, covering all alias types, all member types, multi-VSAN, and edge cases
**Depends on**: Phase 1
**Requirements**: PARSE-01, PARSE-02, PARSE-03, PARSE-04, PARSE-05, PARSE-06
**Success Criteria** (what must be TRUE):
  1. Given a real NX-OS running-config with device-alias database, `mds2brocade` prints alias entries to IR without missing or duplicating any entry
  2. Given a multi-VSAN config, all VSANs are parsed and their zones are distinct in the IR (no cross-VSAN merge)
  3. Given a zone with a smart-zoning keyword (`init`/`target`/`both`), the member pWWN is kept and the keyword is stripped with a named warning
  4. Given a zone with an unsupported member type (interface, fcid, ip-address), a named warning is emitted and the member is skipped — the zone still appears in IR
  5. Given an NX-OS 8.5+ config with enhanced device-alias mode, zone members that reference device-alias names are correctly resolved to pWWNs via two-pass parsing
**Plans**: 2 plans

Plans:
- [x] 02-01-PLAN.md — Six NX-OS test fixture files (TDD prerequisite: basic, enhanced_mode, multi_vsan, smart_zoning, unsupported, edge_cases)
- [ ] 02-02-PLAN.md — Two-pass MDS parser state machine (parser.go) and table-driven tests (parser_test.go)

### Phase 3: Brocade Parser
**Goal**: The Brocade parser correctly reads both cfgshow output format and FOS CLI script format, auto-detecting the format, and produces a fully populated IR struct
**Depends on**: Phase 1
**Requirements**: PARSE-07, PARSE-08, PARSE-09
**Success Criteria** (what must be TRUE):
  1. Given a Brocade cfgshow output with wrapped member lines (backslash continuation), all aliases, zones, and cfgs are parsed without truncation
  2. Given a FOS CLI script with `alicreate`, `zonecreate`, and `cfgcreate` commands, the parser produces IR equivalent to what cfgshow would produce for the same config
  3. Given either format as input, format auto-detection selects the correct parser without user-provided flags
**Plans**: TBD

### Phase 4: Validator and Sanitizer
**Goal**: Every name in the IR that would produce invalid or silently broken Brocade output is caught, sanitized, and warned about before any emitter runs
**Depends on**: Phase 2, Phase 3
**Requirements**: SANI-01, SANI-02, SANI-03
**Success Criteria** (what must be TRUE):
  1. Given an alias name longer than 63 characters, the tool truncates it and emits a warning showing the old name and the new name
  2. Given an alias name containing a hyphen (pre-8.1 mode), the hyphen is replaced with underscore and a per-name warning is emitted
  3. Given two MDS names that produce the same sanitized Brocade name, a collision warning is emitted listing all affected original names, and the output names are disambiguated
  4. With `--fos-version 8.1+`, dollar and caret characters are permitted and no warning is emitted for them
**Plans**: TBD

### Phase 5: Brocade Emitter
**Goal**: The Brocade emitter produces correct, ready-to-apply FOS CLI commands from a validated IR, including mandatory security and persistence preamble/postamble
**Depends on**: Phase 4
**Requirements**: CONV-01, CONV-02, CONV-03, OUT-01, OUT-02
**Success Criteria** (what must be TRUE):
  1. Given a populated IR, stdout contains `alicreate` commands for every alias, `zonecreate` for every zone, and `cfgcreate` for every cfg — in that order
  2. Every generated shell script starts with `defzone --noaccess` and ends with `cfgsave` with no option to omit either
  3. `cfgenable` appears in the generated script as a commented-out line with an explanatory comment, never as an executable statement
  4. Given a zone whose member is referenced by alias name, the emitted `zonecreate` command lists the alias name (not the raw pWWN) correctly
**Plans**: TBD

### Phase 6: MDS Emitter
**Goal**: The MDS emitter produces correct NX-OS CLI commands from a validated IR for the brocade2mds direction
**Depends on**: Phase 4
**Requirements**: CONV-04, CONV-05, CONV-06, OUT-03
**Success Criteria** (what must be TRUE):
  1. Given a Brocade IR, the emitter produces a `device-alias database` block with one `device-alias name X pwwn Y` line per alias
  2. Given a Brocade IR, the emitter produces `zone name X vsan 1` blocks with correct `member` lines for each zone
  3. Given a Brocade IR, the emitter produces a `zoneset name X vsan 1` block followed by `zoneset activate name X vsan 1`
**Plans**: TBD

### Phase 7: CLI Wiring and Integration
**Goal**: The complete san-conv pipeline is wired end-to-end with all user-facing flags operational, summary output to stderr, and a distributable cross-platform binary
**Depends on**: Phase 5, Phase 6
**Requirements**: CLI-01, CLI-02, CLI-03, CLI-04, CLI-05, CLI-06, OUT-04
**Success Criteria** (what must be TRUE):
  1. `san-conv myconfig.txt` (positional argument) runs the default mds2brocade conversion and writes FOS commands to stdout
  2. `san-conv myconfig.txt --output result.txt --script result.sh` writes FOS commands to result.txt and a runnable script to result.sh
  3. After any conversion, stderr contains a summary line showing counts of objects converted, objects skipped, and warnings issued
  4. `san-conv myconfig.txt --direction brocade2mds` produces NX-OS commands for a valid Brocade input file
  5. When a fatal IO error occurs the tool exits non-zero; when only warnings occur the tool exits 0
  6. `go install` or a downloaded binary runs on Linux, macOS, and Windows without installing Go or any runtime
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 1 → 2 → 3 → 4 → 5 → 6 → 7

| Phase | Plans Complete | Status | Completed |
|-------|----------------|--------|-----------|
| 1. Foundation | 1/2 | Complete    | 2026-03-29 |
| 2. MDS Parser | 1/2 | In Progress|  |
| 3. Brocade Parser | 0/TBD | Not started | - |
| 4. Validator and Sanitizer | 0/TBD | Not started | - |
| 5. Brocade Emitter | 0/TBD | Not started | - |
| 6. MDS Emitter | 0/TBD | Not started | - |
| 7. CLI Wiring and Integration | 0/TBD | Not started | - |
