# Feature Research

**Domain:** SAN fabric zoning configuration converter (Cisco MDS NX-OS to/from Brocade FOS)
**Researched:** 2026-03-28
**Confidence:** HIGH (vendor docs, GitHub tools, real-world conversion guides)

---

## Feature Landscape

### Table Stakes (Users Expect These)

Features ops teams assume a zoning converter provides. Missing any of these makes the tool feel broken or untrustworthy.

| Feature | Why Expected | Complexity | Notes |
|---------|--------------|------------|-------|
| Parse Cisco MDS `show running-config` zoning sections | Primary input format; tool is worthless without it | MEDIUM | Must handle `device-alias database`, `zone name`, `zoneset name`, `zoneset activate` blocks; multi-VSAN configs common |
| Parse Brocade FOS `cfgshow` / `configshow` output | Bidirectional input; also needed for Brocade→MDS direction | MEDIUM | Defined vs Effective config sections; alias/zone/cfg blocks separated by semicolons |
| Convert device-alias (MDS) → alias/alicreate (Brocade) | Core object; everything else references aliases | LOW | Both platforms use 64-char max; Cisco limits to alphanumeric+underscore+hyphen; Brocade (pre-8.1) allows only alphanumeric+underscore — dashes silently dropped in FOS CLI |
| Convert zone members (pWWN) preserving WWN identity | Zone membership fidelity is non-negotiable | LOW | Both platforms use `xx:xx:xx:xx:xx:xx:xx:xx` colon-hex format; WWN normalization to lowercase canonical form required |
| Convert zone definitions → zonecreate / alicreate | Core output object | LOW | Members joined with semicolons in FOS; one `member` line each in NX-OS |
| Convert zoneset (MDS) → cfg / cfgcreate (Brocade) | Contains the active zoning policy | LOW | MDS has one active zoneset per VSAN; FOS has one enabled cfg per fabric |
| Output ready-to-paste FOS CLI commands | Ops team pastes directly into switch CLI | LOW | Output must be syntactically correct; cfgsave and cfgenable must be included |
| Output executable shell script | Ops team may need to run non-interactively | LOW | Wrap FOS CLI commands in a script with cfgsave + cfgenable at end |
| Warn on unconvertible constructs and continue | Partial output is better than a hard stop | LOW | Warn to stderr; continue producing as much valid output as possible |
| CLI flag: conversion direction (mds2brocade / brocade2mds) | Bidirectional tool needs explicit mode selection | LOW | Default to mds2brocade since that is the primary use case |
| Single distributable binary, no runtime deps | Ops teams do not manage Python/Node environments | N/A | Go satisfies this; already decided in PROJECT.md |
| Handle comments and blank lines in input | Real configs have documentation comments | LOW | NX-OS uses `!`; FOS uses `#` — strip both silently |
| Handle multi-VSAN MDS configs | Production MDS switches routinely have 2-16 VSANs | MEDIUM | Each VSAN is a separate zoning domain; each produces a separate Brocade cfg block |

### Differentiators (Competitive Advantage)

Features that set this tool apart from the existing Cisco ZoneMigrator (Windows-only .exe, pWWN-only, FOS 7/8 only, no support).

| Feature | Value Proposition | Complexity | Notes |
|---------|-------------------|------------|-------|
| Cross-platform binary (Linux/macOS/Windows) | Existing Cisco tool is Windows 7/10 only; ops teams run Linux/macOS | LOW | Go cross-compilation is trivial; publish releases for all three |
| Support for FOS 9.x and FOS 10.x | Cisco tool only tested on FOS 7/8; many shops are on FOS 9+ | LOW | Naming rules changed in FOS 8.1+ (hyphen, dollar, caret allowed; numeric-start allowed); validate against current FOS docs |
| Name-conflict detection and deduplication | Both platforms share 64-char limit but different allowed character sets; silent collision possible | MEDIUM | Detect when alias/zone names collide after character substitution; emit named warning with both original and sanitized names |
| Character sanitization with explicit warnings | FOS CLI silently drops dashes — a real-world data-loss trap confirmed by operators | LOW | Translate disallowed chars to underscore; warn per occurrence; do NOT silently ignore like FOS does |
| Structured conversion summary to stderr | Ops teams need to audit what was converted, skipped, or warned | LOW | Print counts: aliases converted, zones converted, zonesets converted, warnings, skipped objects |
| Detect and report unsupported zone member types | MDS supports fcid, interface, ip-address, fwwn, symbolic-nodename members — none have FOS equivalents | LOW | Hard zone (domain,port) also has no direct pWWN equivalent; flag each with specific message |
| Detect orphaned zones (defined but not in any zoneset) | Cisco tool migrates all defined objects including dead zones; conversion is a good time to surface this | LOW | Emit warning listing orphaned zone names; still convert them (ops team decides) |
| Per-VSAN output files for multi-VSAN configs | Large MDS configs may have 10+ VSANs; one output file per VSAN aids incremental migration | MEDIUM | Flag: `--split-vsan` or default behavior; name output files `fabric_vsanNNN.sh` |
| Roundtrip fidelity documentation | No existing tool documents what survives conversion and what does not | LOW | Not a code feature — document conversion map in README |
| Device-alias basic mode expansion detection | MDS in basic mode stores pWWN not alias name in zone members; tool must detect and reconstruct alias references | MEDIUM | In basic mode, zone members show as `member pwwn xx:xx:...`; must cross-reference device-alias database to recover alias names for output |

### Anti-Features (Commonly Requested, Often Problematic)

| Feature | Why Requested | Why Problematic | Alternative |
|---------|---------------|-----------------|-------------|
| SSH / live switch connection | Automate the apply step | Scope explosion, auth complexity, risk of misconfiguration on production fabric; violates "offline tool" design contract | Generate script; ops team applies; human review is the safety gate |
| GUI or web interface | More accessible | Contradicts single-binary ops model; ops teams trust CLI tools for prod config tasks | Good CLI flag design with `--help`; clear stdout/stderr separation |
| VSAN-to-fabric topology mapping | Automate which FOS fabric each VSAN maps to | Requires topology knowledge that cannot be derived from a config file alone; must be a human decision | Produce per-VSAN output; operator assigns each to the correct fabric |
| Zone enforcement mode translation (hard/soft) | Preserve security posture | FOS and NX-OS semantics differ fundamentally; enforcing "hard" mode on FOS has different switch-level implications | Warn that enforcement mode was not carried across; document in output header comment |
| Traffic Isolation Zone (TI zone) conversion | TI zones exist in some FOS configs | TI zones deprecated in FOS 9.0; no equivalent in NX-OS; converting them would produce invalid output | Detect TI zones in FOS input; emit warning "TI zones deprecated in FOS 9.0 and have no NX-OS equivalent — skipped" |
| Peer zone / Target-Driven Peer Zone conversion | Peer zones exist in FOS configs | No direct NX-OS equivalent; Smart Zoning (Cisco) is loosely analogous but requires different configuration mechanism | Detect peer zones; emit warning; optionally convert to standard zones with a flag |
| QoS zone prefix handling | Preserve QoS priority assignments | QoS zone naming (`QOS_H_`, `QOS_L_`) is FOS-specific; NX-OS uses different QoS mechanisms | Strip QoS prefix; warn that QoS attributes were not preserved; document separately |
| Automatic zoneset activation in output script | Convenience | cfgenable disrupts traffic on fabric — this decision must be explicit and human-controlled | Generate cfgenable as a commented-out line with a prominent warning comment |
| Interactive prompts during conversion | Friendlier UX | Breaks automation pipelines (`cat config | san-conv`); ops teams script this | Use flags for all decisions; fail fast with clear error messages |
| Config validation against live switch | Verify converted config is consistent with current fabric state | Requires live connectivity; out-of-scope for offline tool | Produce syntactically valid output; let the switch CLI reject invalid constructs on apply |

---

## Feature Dependencies

```
[Parse MDS config]
    └──requires──> [Produce any MDS→Brocade output]
                       ├──requires──> [Device-alias conversion]
                       ├──requires──> [Zone conversion]
                       └──requires──> [Zoneset→cfg conversion]
                                          └──requires──> [Zone conversion]

[Parse Brocade config]
    └──requires──> [Produce any Brocade→MDS output]
                       ├──requires──> [alias→device-alias conversion]
                       ├──requires──> [zonecreate→zone conversion]
                       └──requires──> [cfgcreate→zoneset conversion]

[Name sanitization]
    └──enhances──> [Device-alias conversion]
    └──enhances──> [Zone conversion]

[Name-conflict detection]
    └──requires──> [Name sanitization]

[Multi-VSAN handling]
    └──enhances──> [Parse MDS config]
    └──enhances──> [Per-VSAN output files]

[Conversion summary to stderr]
    └──enhances──> [All conversion steps] (aggregates counts)

[Unsupported member type detection]
    └──requires──> [Zone conversion] (fires during zone member processing)

[Orphaned zone detection]
    └──requires──> [Zone conversion]
    └──requires──> [Zoneset→cfg conversion] (cross-reference needed)
```

### Dependency Notes

- **Zone conversion requires device-alias conversion first:** Zone members may reference alias names; the alias mapping must be built before zones are processed.
- **Zoneset conversion requires zone conversion:** cfgcreate lists zone names; zones must exist in the output before the cfg is emitted.
- **Name sanitization enhances alias/zone conversion:** Must run as part of name assignment, not as a post-processing step.
- **Multi-VSAN handling requires parser to bucket objects by VSAN:** MDS config interleaves VSAN contexts; parser must track current VSAN scope throughout.
- **Basic mode detection requires alias DB cross-reference:** If zone members are stored as pWWN (basic mode), alias names must be recovered from the device-alias database before FOS alicreate commands can be generated.

---

## MVP Definition

### Launch With (v1)

Minimum viable product — what ops teams need to trust and use the tool.

- [ ] Parse full MDS `show running-config` extracting device-alias database, zone definitions, zoneset definitions, active zoneset — **why essential: primary input format**
- [ ] Parse Brocade `cfgshow` / `configshow` output extracting aliases, zones, cfgs — **why essential: bidirectional support**
- [ ] Convert device-alias → alicreate with character sanitization and per-name warnings — **why essential: all zone membership hangs on alias names**
- [ ] Convert pWWN zone members with WWN normalization to lowercase colon-hex — **why essential: WWN mismatches cause zone membership failures**
- [ ] Convert zones → zonecreate — **why essential: core object**
- [ ] Convert active zoneset → cfgcreate + cfgenable (commented out) — **why essential: without the cfg, zones exist but do nothing**
- [ ] Output FOS CLI commands to stdout — **why essential: paste-to-switch workflow**
- [ ] Output executable shell script via `--output` flag — **why essential: script workflow**
- [ ] Conversion direction flag (`--direction mds2brocade|brocade2mds`) — **why essential: bidirectional tool**
- [ ] Warn-and-continue on all errors; structured warning output to stderr — **why essential: partial output beats hard stop for ops review**
- [ ] Conversion summary: counts of converted/warned/skipped objects — **why essential: ops team needs audit trail**
- [ ] Detect unsupported zone member types (fcid, interface, ip-address, fwwn, domain-port) and emit named warnings — **why essential: silent data loss is worse than a warning**
- [ ] Handle multi-VSAN MDS config — **why essential: production MDS switches always have multiple VSANs**

### Add After Validation (v1.x)

Features to add once core conversion is confirmed accurate on real configs.

- [ ] Per-VSAN output file splitting (`--split-vsan`) — trigger: ops teams report multi-VSAN configs are hard to manage as one file
- [ ] Orphaned zone detection and reporting — trigger: first real-world config with stale zones surfaces the need
- [ ] Device-alias basic mode expansion detection and alias-name recovery — trigger: ops report pWWN-only zone members in output when alias names were expected
- [ ] Peer zone detection and warning (with optional flatten-to-standard-zone flag) — trigger: FOS configs with peer zones encountered

### Future Consideration (v2+)

Features to defer until product-market fit is established.

- [ ] LSAN zone handling (cross-fabric zones) — defer: requires understanding of IVR/metaSAN topology
- [ ] QoS zone prefix stripping with optional preservation mode — defer: few environments use FOS QoS zoning
- [ ] JSON/YAML intermediate representation output (`--format json`) — defer: machine-readable output useful for pipeline integration, but not needed for manual ops workflow
- [ ] Config diff mode: compare two zone databases and output delta — defer: useful for incremental migrations but adds significant complexity

---

## Feature Prioritization Matrix

| Feature | User Value | Implementation Cost | Priority |
|---------|------------|---------------------|----------|
| Parse MDS running-config | HIGH | MEDIUM | P1 |
| Parse Brocade cfgshow output | HIGH | MEDIUM | P1 |
| device-alias → alicreate conversion | HIGH | LOW | P1 |
| pWWN zone member conversion + WWN normalization | HIGH | LOW | P1 |
| zone → zonecreate conversion | HIGH | LOW | P1 |
| zoneset → cfgcreate conversion | HIGH | LOW | P1 |
| FOS CLI stdout output | HIGH | LOW | P1 |
| Shell script output (--output flag) | HIGH | LOW | P1 |
| Conversion direction flag | HIGH | LOW | P1 |
| Warn-and-continue on errors | HIGH | LOW | P1 |
| Conversion summary counts | HIGH | LOW | P1 |
| Multi-VSAN MDS config handling | HIGH | MEDIUM | P1 |
| Unsupported member type detection | MEDIUM | LOW | P1 |
| Character sanitization with warnings | HIGH | LOW | P1 |
| Name-conflict detection | MEDIUM | MEDIUM | P2 |
| Cross-platform binary (Linux/macOS/Windows) | HIGH | LOW | P1 |
| Per-VSAN output splitting | MEDIUM | MEDIUM | P2 |
| Orphaned zone detection | MEDIUM | LOW | P2 |
| Basic mode alias-name recovery | MEDIUM | MEDIUM | P2 |
| Peer zone detection and warning | LOW | LOW | P2 |
| JSON/YAML output format | LOW | MEDIUM | P3 |
| LSAN zone handling | LOW | HIGH | P3 |
| Config diff mode | MEDIUM | HIGH | P3 |

**Priority key:**
- P1: Must have for launch
- P2: Should have, add when possible
- P3: Nice to have, future consideration

---

## Competitor Feature Analysis

| Feature | Cisco ZoneMigrator (GitHub) | Broadcom SAN Health Zone Migration | san-conv (this tool) |
|---------|-----------------------------|------------------------------------|----------------------|
| pWWN zone conversion | Yes | Yes | Yes |
| Hard zone (domain,port) support | No (errors out) | Unknown | Warn + skip |
| Alias/device-alias conversion | Yes (fcalias or device-alias) | Yes | Yes (device-alias enhanced mode) |
| Multi-VSAN handling | Yes (per-VSAN VSAN index setting) | Unknown | Yes |
| Platform support | Windows 64-bit only | Any (SAN Health client) | Linux / macOS / Windows |
| FOS version tested | FOS 7.x and 8.x only | Unknown | FOS 7.x–10.x (naming rule aware) |
| Distribution model | .exe download or go install | Requires emailing SANHealth.Admin@broadcom.com | go install or GitHub release binary |
| Offline operation | Yes (input file) | No (requires Broadcom team involvement) | Yes |
| Conversion direction | Brocade→Cisco only | Brocade→Cisco only | Bidirectional |
| Error handling | Errors to log file, stops | Vendor-managed process | Warn and continue |
| Conversion summary | Log file | Vendor-provided report | Structured stderr output |
| Character sanitization warnings | No (silent) | Unknown | Explicit per-name warnings |
| Open source | Yes (no support) | No (proprietary service) | Yes |

---

## Key Naming Constraint Reference

Conversion must account for different naming rules on each platform:

| Constraint | Cisco MDS NX-OS | Brocade FOS (pre-8.1) | Brocade FOS (8.1+) |
|------------|----------------|----------------------|-------------------|
| Max name length | 64 chars (63 chars from NX-OS 9.2.2 for device-alias) | 64 chars | 64 chars |
| Start with letter required | Yes | Yes | No (numeric start allowed) |
| Underscore | Yes | Yes | Yes |
| Hyphen | Yes (NX-OS allows) | No (CLI silently drops!) | Yes |
| Dollar sign | No | No | Yes |
| Caret | No | No | Yes |
| Case-sensitive | Yes | Yes | Yes |
| Reserved prefixes | None documented | `bfa_`, `lsan_red_`, `d__efault__` | Same plus `broadcast` (FOS 9.0.1+) |

**Critical implication:** When converting MDS→Brocade for pre-8.1 FOS targets, hyphens in Cisco names must be translated to underscores with an explicit warning. The FOS CLI silently accepts and ignores hyphens — this caused real production outages (confirmed by operator reports).

---

## Sources

- [GitHub - Cisco-SAN/ZoneMigrator](https://github.com/Cisco-SAN/ZoneMigrator) — Cisco's official zone migration tool (Brocade→Cisco); Windows-only, pWWN-only, FOS 7/8 tested
- [Dell KB: Cisco to Brocade Migration using Brocade SAN Health](https://www.dell.com/support/kbdoc/en-us/000184651/connectrix-b-series-cisco-to-brocade-migration-how-to-import-the-zoning-from-the-cisco-switch-to-the-brocade-switch) — Broadcom's SAN Health service-based migration process
- [Broadcom FOS 9.2.x Zone Types](https://techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-administration/9-2-x/Administering-Advanced-Zoning-AG/v26770744.html) — Authoritative FOS zone type enumeration
- [Broadcom FOS 9.2.x Peer Zoning](https://techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-administration/9-2-x/Administering-Advanced-Zoning-AG/v26773885.html) — Peer zone structure
- [Brocade Zone Name Restrictions (ManualsLib)](https://www.manualslib.com/manual/21614/Brocade-Communications-Systems-53-1001763-02.html?page=347) — Legacy FOS naming rules
- [Dell KB: Enhanced Zone Object naming not supported](https://www.dell.com/support/kbdoc/en-us/000227366/connectrix-brocade-unable-to-create-alias-zones-due-to-error-enhanced-zone-object-naming-feature-is-not-supported-by-the-fabric) — FOS 8.1+ naming feature compatibility
- [PenguinPunk: Brocade alias and zone syntax gotchas](https://www.penguinpunk.net/blog/brocade-alias-and-zone-syntax-or-how-fos-is-a-love-hate-thing/) — Real-world confirmation that FOS CLI silently drops dashes (production outage case study)
- [Cisco MDS 9.x Zone Configuration Guide](https://www.cisco.com/c/en/us/td/docs/dcn/mds9000/sw/9x/configuration/fabric/cisco-mds-9000-nx-os-fabric-configuration-guide-9x/configuring_and_managing_zones.html) — NX-OS zone member types, naming rules
- [Cisco MDS 9.x Device Alias Guide](https://www.cisco.com/c/en/us/td/docs/dcn/mds9000/sw/9x/configuration/fabric/cisco-mds-9000-nx-os-fabric-configuration-guide-9x/distributing_device_alias_services.html) — device-alias basic vs enhanced mode, 63-char limit in 9.2.2+
- [Nick Tailor: Cisco vs Brocade SAN Switch Commands](https://nicktailor.com/tech-blog/cisco-vs-brocade-san-switch-commands-explained-with-diagnostics-and-examples/) — Command equivalence reference with WWN format examples
- [Dell KB: TI Zone creation no longer supported FOS 9.0](https://www.dell.com/support/kbdoc/en-us/000216258/connectrix-brocade-error-traffic-isolation-zone-creation-and-editing-is-no-longer-supported-in-this-version-of-fabric-os) — TI zone deprecation in FOS 9.0
- [Cisco Smart Zoning](https://www.cisco.com/c/en/us/support/docs/storage-networking/zoning/116390-technote-smartzoning-00.html) — MDS Smart Zoning vs Brocade Peer Zoning analogy
- [IBM SAN Zoning Best Practices](https://www.ibm.com/support/pages/san-zoning-best-practices) — Orphaned zone cleanup guidance during migration

---
*Feature research for: SAN fabric zoning configuration converter (Cisco MDS NX-OS ↔ Brocade FOS)*
*Researched: 2026-03-28*
