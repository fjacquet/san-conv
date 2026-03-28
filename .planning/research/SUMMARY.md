# Project Research Summary

**Project:** san-conv
**Domain:** Go CLI tool — bidirectional SAN zoning configuration converter (Cisco MDS NX-OS / Brocade FOS)
**Researched:** 2026-03-28
**Confidence:** HIGH

## Executive Summary

san-conv is a single-binary Go CLI tool for converting SAN fabric zoning configurations between Cisco MDS NX-OS and Brocade FOS formats. The tool fills a real gap: Cisco's official ZoneMigrator is Windows-only, pWWN-only, untested beyond FOS 8.x, and Brocade→Cisco direction only. The recommended architecture follows the compiler front-end/back-end pattern — each vendor format has a dedicated parser producing a format-neutral intermediate representation (IR), and each vendor has a dedicated emitter consuming that IR. This structure keeps the tool bidirectional, testable, and extensible at minimal complexity cost.

The recommended implementation is a pure-stdlib Go tool with one external runtime dependency (cobra for CLI structure). Parsing uses `bufio.Scanner` with a state-machine and compiled regexes; output generation uses `text/template`. This approach is well-documented, has no runtime dependencies for ops team distribution, and compiles to a single binary for Linux, macOS, and Windows with trivial cross-compilation. The tool targets Go 1.25 with `log/slog` for structured warnings and uses `goreleaser` for release automation.

The critical risk category is silent data corruption during conversion. Brocade FOS pre-8.1 silently strips hyphens from alias names — confirmed to have caused production outages. Name collisions after character sanitization, missing `cfgsave` in generated scripts, and incorrect default zone policy (`allaccess` vs MDS `deny`) are all pitfalls that can produce a working-looking but subtly broken fabric. Every one of these is preventable by design: the sanitization module, output template, and validator must address them before any output reaches an operator.

---

## Key Findings

### Recommended Stack

The stack is minimal by design — a deliberate "no surprises" foundation appropriate for an ops tool that ops teams must trust. The only external runtime dependency is cobra; everything else is Go stdlib. Dev tooling (golangci-lint v2, goreleaser v2, cobra-cli) is managed via Go 1.24+ `tool` directives in `go.mod`, eliminating the `tools.go` workaround. Go 1.25 is current stable (1.25.8 as of March 2026).

**Core technologies:**
- **Go 1.25:** Single-binary compilation, no runtime deps, cross-platform — project constraint; perfect fit
- **github.com/spf13/cobra v1.10.2:** CLI subcommands (`mds2brocade`, `brocade2mds`), flags, `--help`, shell completion — industry standard
- **stdlib `bufio` + `regexp`:** Line-by-line parsing with state machine — canonical Go approach for vendor config text; no grammar library fits this domain
- **stdlib `text/template`:** FOS/NX-OS CLI command generation — separates output format from conversion logic; critical for multi-format extensibility
- **stdlib `log/slog`:** Structured warnings to stderr — built-in since Go 1.21; zero external dep; correct tool for warn-and-continue diagnostics
- **github.com/stretchr/testify v1.11.1:** Test assertions with `require` package — stops test execution on first failure, correct for parser unit tests

**What NOT to use:** `html/template` (corrupts FOS output via HTML escaping), logrus (maintenance-mode), Viper (no persistent config needed), any Python-based parser, `go-nxos` (REST client, not file parser).

### Expected Features

The MVP addresses ops teams migrating production SAN fabrics. The tool must be trustworthy before it is feature-complete — partial output with explicit warnings beats a hard stop or silent data loss.

**Must have (table stakes — P1):**
- Parse full MDS `show running-config` zoning sections (device-alias database, zones, zonesets, multi-VSAN)
- Parse Brocade `cfgshow` / `configshow` output (including wrapped member lists)
- Convert device-alias → alicreate with character sanitization and per-name warnings
- Convert pWWN zone members with WWN normalization to lowercase colon-hex
- Convert zones → zonecreate; zoneset → cfgcreate + cfgenable (commented out)
- Output FOS CLI commands to stdout; optional shell script via `--output` flag
- Bidirectional direction flag (`--direction mds2brocade|brocade2mds`)
- Warn-and-continue on all unconvertible constructs; structured warning output to stderr
- Conversion summary (counts: aliases/zones/zonesets converted, warned, skipped)
- Detect unsupported zone member types (fcid, interface, ip-address, fwwn, domain-port) with named warnings
- Handle multi-VSAN MDS configs (VSAN-scoped parsing)
- Cross-platform binary: Linux/macOS/Windows

**Should have (differentiators — P2):**
- Name-conflict detection and deduplication (collision after sanitization)
- Per-VSAN output file splitting (`--split-vsan`)
- Orphaned zone detection and reporting
- Device-alias basic mode: pWWN zone member → alias name recovery

**Defer (v2+):**
- JSON/YAML intermediate representation output (`--format json`)
- LSAN zone handling (cross-fabric zones requiring IVR/metaSAN topology)
- Config diff mode (delta between two zone databases)

**Anti-features (explicitly out of scope):**
- SSH/live switch connection — violates offline tool contract, unacceptable production risk
- GUI or web interface — contradicts single-binary ops model
- Interactive prompts — breaks automation pipelines
- Automatic cfgenable activation — must be commented out; human decision required

### Architecture Approach

The architecture follows a strict compiler pipeline: parser (frontend) → canonical IR → validator → emitter (backend). Each vendor has a dedicated parser and emitter. The IR (`ZoningConfig` struct in `internal/ir/`) is the only shared data contract; all other packages depend on it but not on each other. This produces a strict DAG with no import cycles and makes bidirectionality O(n) rather than O(n²) as formats are added. All emitters accept `io.Writer` — never `os.Stdout` directly — making output testable and flaggable to files without rewrites.

**Major components:**
1. `cmd/san-conv/cmd/` (cobra CLI) — wires pipeline, owns flag definitions, opens files, orchestrates parse→validate→emit
2. `internal/parser/mds` and `internal/parser/brocade` — vendor-specific state-machine parsers producing `*ir.ZoningConfig`
3. `internal/ir/zoningconfig.go` — format-neutral canonical struct definitions; zero logic; no import cycles possible
4. `internal/validator/validator.go` — reads IR, emits `[]Warning`; never mutates IR; never errors
5. `internal/emitter/brocade` and `internal/emitter/mds` — template-driven CLI command renderers writing to `io.Writer`
6. `testdata/mds/` and `testdata/brocade/` — real fixture config files for table-driven and golden-file tests

**Build order mandated by dependencies:** IR first → parsers (can be parallel with emitters once IR is stable) → validator → emitters → CLI wiring → integration tests.

### Critical Pitfalls

Fifteen pitfalls were identified; the following five have HIGH recovery cost or are most likely to be missed:

1. **Device-alias enhanced mode silently drops all zone members** — NX-OS 8.5.1+ stores zone members as `member device-alias NAME`, not `member pwwn`. A parser that only handles pWWN members silently produces empty zones. Prevention: two-pass parser (pass 1 builds alias DB, pass 2 resolves zone members); test with enhanced-mode fixture from NX-OS 8.5+ switch.

2. **Brocade FOS pre-8.1 silently strips hyphens from alias names** — FOS CLI accepts `alicreate "HOST-HBA1"` without error but creates `HOSTHBA1`; zone membership silently breaks post-activation. Prevention: sanitization module replaces `-` with `_` and emits a per-name warning before any output is generated.

3. **Enhanced zone naming is a fabric-wide capability** — Even if the target switch runs FOS 8.1+, one older switch in the fabric causes `FABR-1001` errors on any name using hyphens/dollar/caret. Prevention: default to the conservative character set (letters, digits, underscore; start with letter); add `--target-fos-version 8.1+` flag to opt into expanded naming.

4. **Missing `cfgsave` causes non-persistent Brocade zoning** — Unlike Cisco `zoneset activate` (which persists), Brocade `cfgenable` is volatile. Without `cfgsave`, a reboot reverts all zoning. Prevention: `cfgsave` is a mandatory final line in every generated FOS script template; validator warns if absent.

5. **Default zone policy mismatch creates security exposure** — MDS default is `deny`; some Brocade platforms/versions default to `allaccess`. Prevention: prepend `defzone --noaccess` to every generated FOS script; never make this optional.

Additional high-importance pitfalls: name length collision after sanitization (two MDS names can sanitize to the same Brocade name — requires post-sanitization collision detection pass), multi-VSAN VSAN-scoped parsing (naive parsers merge zones across VSANs), smart zoning keywords (`init`/`target`/`both` appended to member pWWN lines — must be stripped), IVR zones misidentified as regular zones (must be explicitly excluded), and Brocade cfgshow wrapped member lists (continuation lines must be joined before parsing).

---

## Implications for Roadmap

Based on the dependency graph from ARCHITECTURE.md and pitfall-to-phase mapping from PITFALLS.md:

### Phase 1: Foundation — IR and Project Scaffolding
**Rationale:** The IR struct is the shared contract that all other packages depend on. Changing it after parsers and emitters exist cascades breaking changes everywhere. It must be stable before any other code is written. Scaffolding (module init, cobra setup, `go.mod` tool directives) has no dependencies and is fastest when done fresh.
**Delivers:** Compilable `san-conv` binary with `mds2brocade` and `brocade2mds` subcommands returning "not implemented"; defined `ZoningConfig`, `Alias`, `Zone`, `ZoneMember`, `ZoneConfig` structs in `internal/ir/`; configured golangci-lint, goreleaser, testdata directory structure.
**Addresses:** Stack setup (cobra, testify, dev tools), IR struct definitions, project layout from ARCHITECTURE.md.
**Avoids:** Import cycles (IR-first design prevents them by construction).

### Phase 2: MDS Parser — NX-OS Running-Config Parsing
**Rationale:** MDS→Brocade is the primary direction. The MDS parser is the most complex component (multi-VSAN, two alias types, multiple member types, smart zoning) and contains the most pitfalls. Building and thoroughly testing it before touching emitters ensures the IR is well-exercised before downstream components depend on it.
**Delivers:** `internal/parser/mds` that correctly parses device-alias database, fcalias blocks, zone definitions (all member types), and zoneset definitions from real NX-OS running-configs; table-driven tests with fixture files covering enhanced mode, basic mode, multi-VSAN, smart zoning, IVR zones, and edge cases.
**Implements:** Parser state machine pattern, two-pass alias resolution, VSAN-scoped zone buckets.
**Avoids:** Pitfalls 1 (enhanced mode), 4 (fcalias coexistence), 6 (multi-VSAN merge), 7 (smart zoning keywords), 10 (IVR zone misidentification), 13 (non-WWN member types), 15 (inactive zones silently omitted).

### Phase 3: Brocade Parser — FOS cfgshow Parsing
**Rationale:** Brocade parser can be built in parallel with the MDS parser (same IR target, no inter-dependency) but is placed here sequentially to allow full focus on one parser at a time. The Brocade parser has its own set of format-specific pitfalls distinct from MDS.
**Delivers:** `internal/parser/brocade` that correctly parses cfgshow output (defined and effective sections) and FOS CLI script format; handles wrapped member lists; normalizes space-in-name; table-driven tests with fixture files.
**Avoids:** Pitfalls 8 (spaces ignored in zone names), 14 (wrapped member list parsing).

### Phase 4: Validator and Name Sanitization
**Rationale:** The sanitization and validation logic must be built before emitters, because emitters render whatever is in the IR — an unsanitized name in the IR will produce invalid FOS output. The validator is also intentionally read-only (no IR mutation), and establishing this boundary early prevents the anti-pattern of "fixing" names inside validation.
**Delivers:** `internal/validator/validator.go` with name-length checks (63-char limit), character set validation (hyphen detection, FOS pre-8.1 rules), WWN format validation, post-sanitization collision detection with disambiguation suffixes, empty zone warnings, unsupported member type warnings; `--target-fos-version` flag design.
**Avoids:** Pitfalls 2 (hyphen stripping), 3 (fabric-wide naming requirement), 5 (name collision after sanitization), 11 (63/64-char limit discrepancy).

### Phase 5: Brocade Emitter — FOS CLI Output Generation
**Rationale:** Brocade emitter is the primary output path (primary use case is MDS→Brocade). Template-driven design keeps FOS CLI formatting out of converter logic. Critical security/persistence requirements (`defzone --noaccess`, `cfgsave`) must be embedded in the template, not left as caller responsibility.
**Delivers:** `internal/emitter/brocade` with `text/template`-driven FOS CLI command generation; mandatory `defzone --noaccess` preamble; `cfgsave` as final statement; `cfgenable` commented out with warning comment; golden-file tests comparing template output against expected FOS commands.
**Avoids:** Pitfalls 9 (default zone policy mismatch), 12 (cfgsave omitted).

### Phase 6: MDS Emitter — NX-OS CLI Output Generation
**Rationale:** MDS emitter supports the Brocade→MDS direction. Same patterns as Brocade emitter; placed after to allow lessons from Phase 5 to inform it.
**Delivers:** `internal/emitter/mds` with `text/template`-driven NX-OS CLI command generation (device-alias database block, zone definitions, zoneset definition, `zoneset activate`); golden-file tests.

### Phase 7: CLI Wiring and Integration
**Rationale:** The CLI layer wires the complete pipeline and owns the user-facing features: conversion summary output, stderr/stdout separation, `--output` file flag, and end-to-end integration tests. These cannot be built until all pipeline components exist.
**Delivers:** Complete `san-conv` binary with both subcommands functional end-to-end; conversion summary (counts of converted/warned/skipped objects); structured warning output to stderr; `--output` flag for script file generation; integration tests using real fixture configs; goreleaser build producing Linux/macOS/Windows binaries.
**Addresses:** All P1 features from FEATURES.md; UX pitfalls (stdout/stderr separation, hard-stop avoidance).

### Phase Ordering Rationale

- **IR-first is non-negotiable:** Every package imports `internal/ir`. Changing IR structs after parsers or emitters exist cascades breaking changes. Phase 1 must lock the IR before Phases 2-6 begin.
- **Parsers before emitters in the same direction:** Integration tests require both a complete parser and a complete emitter. Parsers are harder (more pitfalls, more test cases) and should be proven before emitters consume their output.
- **Sanitization/validation before emitters:** The validator must run on IR before it reaches the emitter. Building validation in Phase 4 ensures emitters in Phases 5-6 can be tested against clean, validated IR.
- **CLI wiring last:** The CLI orchestrates components but adds no conversion logic. Building it last means integration tests test real components, not mocks.
- **Brocade emitter before MDS emitter:** MDS→Brocade is the primary use case; build the primary output path first.

### Research Flags

Phases likely needing deeper research during planning:
- **Phase 2 (MDS Parser):** fcalias vs device-alias coexistence in the same zone requires careful test fixture construction from real enterprise configs; smart zoning fixture configs may need sourcing.
- **Phase 4 (Validator / Sanitization):** FOS version matrix (7.x/8.0/8.1+/9.x/10.x) for naming rules may have edge cases not covered in current research; `--target-fos-version` flag semantics need ops team input.

Phases with standard patterns (research-phase likely unnecessary):
- **Phase 1 (Foundation):** Go module setup, cobra scaffolding, goreleaser config — all well-documented with official examples.
- **Phase 5 (Brocade Emitter):** `text/template` rendering is well-understood; FOS CLI command syntax is fully documented in Broadcom official docs.
- **Phase 6 (MDS Emitter):** Same as Phase 5; NX-OS syntax is fully documented in Cisco official docs.
- **Phase 7 (CLI Wiring):** Cobra command wiring is standard; goreleaser cross-compilation is standard.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Core technologies (Go stdlib, cobra, testify) verified against official releases; goreleaser and golangci-lint versions confirmed March 2026. All "what NOT to use" determinations verified. |
| Features | HIGH | Derived from official Cisco and Broadcom docs, real-world operator reports (production outage from hyphen stripping confirmed), and analysis of existing tools (Cisco ZoneMigrator GitHub). Feature boundaries well-justified. |
| Architecture | HIGH | Pattern (parser/IR/emitter pipeline) is academically grounded (CrossTL paper) and consistent with ciscoconfparse's approach; official Go module layout docs consulted; vendor format specs verified against Broadcom and Cisco official documentation. |
| Pitfalls | HIGH | 15 pitfalls identified; critical ones (hyphen stripping, cfgsave, defzone) verified against official vendor docs and confirmed by operator reports. Smart zoning and IVR pitfalls verified against Cisco official documentation. |

**Overall confidence:** HIGH

### Gaps to Address

- **Multi-VSAN output strategy:** The scope decision (one output file vs per-VSAN files) is deferred to v1.x. The v1 behavior for multi-VSAN configs (warn and merge vs warn and error) needs explicit ops team input before implementation — both approaches are valid but have different operational implications.
- **FOS version targeting:** The exact `--target-fos-version` flag design (enum values, defaults, behavior for each value) needs confirmation against the specific FOS versions the ops team's target fabrics run. Research covered the naming rule differences; the UX for selecting them needs refinement.
- **Test fixture availability:** The research recommends testing against "real NX-OS 8.5+ running-configs with enhanced device-alias mode." Actual fixture files from production or lab switches will need to be sourced or synthesized for the parser test suite. Synthetic fixtures are acceptable but must be verified against real switch output.
- **Brocade→MDS direction validation:** The primary use case is MDS→Brocade. The reverse direction (brocade2mds) is architecturally symmetric but less battle-tested in real migrations. Additional research may be warranted on NX-OS import workflows during Phase 6 planning.

---

## Sources

### Primary (HIGH confidence)
- Cisco MDS 9000 NX-OS Fabric Configuration Guide 9.x — device-alias, zones, zonesets, IVR
- Broadcom FOS 9.2.x Administration Guide — zone types, cfgshow format, defzone, cfgenable, cfgsave
- Broadcom FOS 9.2.x Command Reference — alicreate, zonecreate, cfgcreate, cfgenable, defzone
- Go official docs — bufio, text/template, log/slog, modules layout
- Cisco MDS 9000 NX-OS 9.2(2) Release Notes — 63-char device-alias limit
- Dell KB 000227366 — FABR-1001 enhanced zone naming fabric-wide requirement
- Dell KB 000216258 — TI zone deprecation in FOS 9.0
- GitHub Cisco-SAN/ZoneMigrator — existing tool limitations confirmed

### Secondary (MEDIUM confidence)
- PenguinPunk.net — FOS hyphen-stripping confirmed in production (operator report)
- CrossTL research paper (arXiv:2508.21256) — O(n²) → O(n) via unified IR
- golangci-lint v2 migration guide (ldez.github.io) — v2 config format changes
- ciscoconfparse (GitHub) — parent/child config line parsing model
- Nick Tailor blog — Cisco vs Brocade command equivalence reference

### Tertiary (LOW confidence)
- cobra vs kong vs urfave/cli comparison — multiple community sources agree on cobra as default; no single authoritative source
- Brocade alicreate/zonecreate CLI examples (belznotbells.com) — practitioner blog consistent with official docs but not official

---
*Research completed: 2026-03-28*
*Ready for roadmap: yes*
