# Pitfalls Research

**Domain:** SAN zoning config conversion — Cisco MDS (NX-OS) to Brocade FOS (bidirectional)
**Researched:** 2026-03-28
**Confidence:** HIGH (critical pitfalls verified against official Cisco and Broadcom documentation)

---

## Critical Pitfalls

### Pitfall 1: Device-Alias Enhanced Mode — Alias Names in Zone Members, Not pWWNs

**What goes wrong:**
When Cisco MDS runs device-alias in enhanced mode (default since NX-OS 8.5(1)), the running-config shows zone members as `member device-alias HOST_A` — not as `member pwwn 10:00:...`. A parser that only looks for `member pwwn` lines will silently miss all enhanced-mode device-alias zone members, producing an incomplete conversion with no error.

**Why it happens:**
Developers familiar with older MDS configs or lab switches test against basic-mode configs where aliases expand to pWWNs. Enhanced mode is the production default but less common in test fixtures. The two modes produce structurally different running-config output for the same logical configuration.

**How to avoid:**
The parser must handle both zone member forms independently:
- `member pwwn <wwn>` — direct pWWN, resolve immediately
- `member device-alias <name>` — symbolic reference, requires lookup in the `device-alias database` block parsed separately
- `member fcalias <name>` — VSAN-scoped alias, requires lookup in the `fcalias` block for that VSAN

Build a two-pass parser: pass 1 collects the device-alias database and all fcalias definitions; pass 2 resolves zone members. Never assume a zone member is always a raw WWN.

**Warning signs:**
- Conversion output has far fewer alias definitions than expected
- Zone members in output are empty or only a fraction of the input
- Test config uses basic-mode syntax but prod configs do not

**Phase to address:**
Parser implementation phase — define zone member type enum (pWWN, device-alias, fcalias, domain-port, interface) before writing any zone parsing code.

---

### Pitfall 2: Brocade FOS Silently Drops Hyphens in Alias Names (Pre-8.1.0)

**What goes wrong:**
On Brocade switches running FOS prior to 8.1.0, the CLI accepts `alicreate "SAN1-SPA5", "..."` without error but silently strips the hyphen, creating an alias named `SAN1SPA5`. Zone members referencing `SAN1-SPA5` then fail to match, breaking connectivity silently after `cfgenable`.

**Why it happens:**
FOS 8.0.x only allows underscores as special characters. The CLI accepts the command but normalizes the name. The discrepancy between web UI (which rejects hyphens with an error) and CLI (which silently accepts and strips) makes it nearly invisible. Operators only discover it when paths drop post-activation.

**How to avoid:**
During name sanitization, always replace hyphens with underscores in output alias/zone names. Do not rely on the target switch to reject invalid names — it may silently corrupt them instead. Include a sanitization step that:
1. Replaces `-` with `_`
2. Strips or replaces `$`, `^` unless targeting FOS 8.1.0+
3. Ensures names do not start with a digit if targeting FOS 8.0.x and older
4. Emits a warning per renamed object listing original and sanitized name

**Warning signs:**
- Target fabric includes older switches (check FOS version in docs or config comments)
- Source MDS config heavily uses hyphens in device-alias names (common in enterprise naming conventions like `ESX01-HBA1`)
- Post-activation path loss affecting specific aliases with hyphens

**Phase to address:**
Name sanitization module — implement before any output generation; parameterize by target FOS version.

---

### Pitfall 3: Brocade FOS Enhanced Zone Object Naming — Fabric-Wide Requirement

**What goes wrong:**
Starting with FOS 8.1.0, zone/alias names can use hyphens, dollar signs, carets, and can start with a digit. However, this is a fabric-wide capability — if even one switch in the fabric runs FOS 8.0.x or earlier, creating such names generates a `FABR-1001` error and the zone operation fails across the entire fabric.

**Why it happens:**
Engineers assume the target switch's FOS version governs what names are valid. In practice, the most restrictive switch in the fabric governs what the entire fabric accepts.

**How to avoid:**
Default the output sanitization to the most conservative character set (letters, digits, underscore; must start with a letter) unless the operator explicitly confirms all fabric members run FOS 8.1.0+. Provide a CLI flag like `--target-fos-version 8.1+` to unlock enhanced naming. In warning messages, note the constraint.

**Warning signs:**
- Mixed FOS versions mentioned in source config comments or hostname patterns
- Source zone names start with digits or contain hyphens/dollar/caret
- Ops team reports "Enhanced Zone Object naming feature is not supported by the fabric" after applying generated script

**Phase to address:**
Name sanitization module and CLI flag design phase.

---

### Pitfall 4: fcalias vs. device-alias Are Fundamentally Different Objects — Both May Coexist

**What goes wrong:**
MDS configs often contain both `fcalias` (VSAN-scoped, stored in zone server database) and `device-alias` (fabric-wide, stored in a separate global database). A parser that only handles one type silently drops members of the other type. Both can appear as zone members in the same zone: `member fcalias STORAGE_PORT` alongside `member device-alias HOST_A`.

**Why it happens:**
Most documentation and examples show one or the other, never both. Real enterprise configs often contain both because fcalias entries were created years before device-alias was introduced and were never migrated.

**How to avoid:**
Parse both databases. For fcalias: locate `fcalias name <X> vsan <V>` blocks and index by (name, VSAN). For device-alias: locate the global `device-alias database` block and index by name. During zone member resolution, check both indexes. Warn if a member references an alias that is not in either index (orphaned member).

**Warning signs:**
- Source config has a `device-alias database` block near the top and also `fcalias name` blocks inside VSAN sub-sections
- Zone member count in output is lower than in input
- Conversion warnings about unresolved aliases

**Phase to address:**
Parser implementation phase — alias resolution must be a named, explicit step.

---

### Pitfall 5: Name Length Truncation Creates Silent Collisions

**What goes wrong:**
Cisco MDS allows device-alias names up to 64 characters (63 in NX-OS 9.2.2+). Brocade FOS allows zone/alias names up to 64 characters. When long MDS names are sanitized (hyphens replaced with underscores, illegal chars stripped), two distinct MDS names can produce identical Brocade names, creating a silent collision. The second alias overwrites the first, and one set of devices silently gets the wrong zone membership.

**Why it happens:**
Name sanitization is typically implemented as a simple character replacement without checking post-sanitization uniqueness. Collisions only appear when the sanitized namespace is checked as a whole.

**How to avoid:**
After sanitizing all names, run a collision-detection pass over the full name set. For any collision, apply a disambiguation suffix (e.g., append `_1`, `_2`) and emit a warning listing all colliding original names and the chosen output names. Never silently overwrite.

**Warning signs:**
- Source config has many similar long names differing only in characters that get sanitized to the same value
- Warning count in output is lower than expected given the source config size
- Output alias count is lower than input alias count

**Phase to address:**
Name sanitization module — collision detection must be a post-sanitization validation step.

---

### Pitfall 6: Multi-VSAN Configs — Zones and Zonesets Are VSAN-Scoped, Not Global

**What goes wrong:**
A full MDS running-config contains zone definitions, zoneset definitions, and `zoneset activate` statements per VSAN. A parser that reads zone definitions globally (not per-VSAN) will silently merge zones from different VSANs into a single namespace. Since VSAN isolation is out of scope for the tool (per PROJECT.md), the tool must still correctly demultiplex by VSAN and process each independently, warning when multiple VSANs are present.

**Why it happens:**
Simple regex-based parsers scan for `zone name <X>` without tracking which VSAN block they are in. NX-OS config files use `vsan <N>` sub-sections that act as implicit context switches for all subsequent zone/zoneset commands.

**How to avoid:**
Implement a stateful parser that tracks current VSAN context. When a `vsan <N>` (or `zone name <X> vsan <N>`) directive is encountered, update the active VSAN. Scope all zone and zoneset objects to their VSAN. When converting MDS→Brocade, emit a warning if more than one VSAN is present (since Brocade has no VSAN equivalent in the same sense) and either process each VSAN as a separate conversion or merge with explicit warning.

**Warning signs:**
- Source config has `zone name` entries appearing under different VSAN sub-sections
- Zone names in output exceed the expected count
- Two zones in different VSANs have the same name but different members; only one survives

**Phase to address:**
Parser design phase — define the AST/data model before any parsing code.

---

### Pitfall 7: Smart Zoning Keywords (init/target/both) Have No FOS Equivalent

**What goes wrong:**
Cisco MDS smart zoning (introduced in NX-OS 5.2(6)) appends `init`, `target`, or `both` keywords to `member pwwn` lines. A naive parser that splits on whitespace and takes the first two tokens will misparse the pWWN or generate an invalid Brocade member line with the keyword appended to the WWN string.

**Why it happens:**
Smart zoning is an MDS-only feature. Developers unfamiliar with MDS do not know these keywords exist and do not include them in parser test cases.

**How to avoid:**
Parse `member pwwn <wwn> [init|target|both]` with the optional keyword explicitly. Strip the keyword during conversion (it has no Brocade equivalent) and emit a warning: "Smart zoning role annotation on `<alias>` dropped — no FOS equivalent. Verify TCAM behavior on target fabric."

**Warning signs:**
- Source config contains lines like `member pwwn 10:00:... init`
- `zone mode smart-zoning enable` appears in the VSAN context
- Generated Brocade WWN values contain `init`, `target`, or `both` as a suffix

**Phase to address:**
Parser implementation phase — add test fixtures with smart zoning configs.

---

### Pitfall 8: Brocade "Spaces Are Ignored in Zone Names" Is a Parsing Trap

**What goes wrong:**
Broadcom FOS documentation states "spaces are ignored in zone names." This means `zonecreate "Zone 1"` and `zonecreate "Zone1"` are equivalent. A parser reading FOS output that treats quoted strings literally will incorrectly treat `Zone 1` and `Zone1` as different objects. This is relevant when parsing Brocade input for the Brocade→MDS direction.

**Why it happens:**
This behavior is not prominently documented and contradicts most conventions. Developers trust quoted string literals to be exact.

**How to avoid:**
When parsing Brocade FOS config input, normalize all zone/alias/cfg names by stripping internal spaces before indexing. When generating FOS output, never produce names with embedded spaces.

**Warning signs:**
- Brocade input config contains zone names with spaces
- Parsed alias count in Brocade→MDS conversion is lower than `alishow` output suggests (aliases with spaces being counted as duplicates)

**Phase to address:**
Brocade parser implementation phase.

---

### Pitfall 9: Default Zone Policy Semantic Mismatch — Security Exposure on Brocade

**What goes wrong:**
Cisco MDS default zone policy is `deny` (no access) by default — devices not in any zone cannot communicate. Brocade FOS default zone mode defaults to `allaccess` on some platforms/versions, meaning devices not in an active zone configuration can communicate freely. If the generated FOS script activates a cfg without first setting `defzone --noaccess`, the target Brocade fabric may expose unconfigured devices to each other in ways the source MDS fabric did not.

**Why it happens:**
Operators focus on getting the zone configuration right and do not check the default zone policy. The FOS default may differ from what the Brocade switch was set to previously.

**How to avoid:**
Prepend `defzone --noaccess` to every generated FOS script before `cfgenable`. Emit a prominently visible warning in the output: "Default zone policy set to no-access to match MDS deny-by-default behavior. Verify this is correct for your target fabric." This is safe by default — it denies unknown devices rather than permitting them.

**Warning signs:**
- Generated script contains only zonecreate/alicreate/cfgenable with no defzone command
- Ops team reports unexpected device communication after applying the script

**Phase to address:**
Output generation phase — make `defzone --noaccess` a standard preamble in the generated script template.

---

### Pitfall 10: IVR Zones Appear in Running-Config and Cannot Be Converted

**What goes wrong:**
MDS configs with Inter-VSAN Routing (IVR) contain `ivr zone name <X>` and `ivr zoneset name <X>` blocks that look superficially similar to regular zone/zoneset blocks but represent cross-VSAN zoning with no FOS equivalent. A parser that pattern-matches `zone name` will incorrectly parse IVR zone lines as regular zones, producing meaningless or dangerous Brocade output.

**Why it happens:**
The IVR zone and zoneset syntax uses the same keywords as regular zones with an `ivr` prefix. A case-insensitive search for `zone name` will match both. IVR is relatively uncommon in smaller environments but present in large enterprise configs.

**How to avoid:**
Explicitly match `ivr zone` and `ivr zoneset` as separate token types and skip them with a conversion warning: "IVR zone `<name>` skipped — Inter-VSAN Routing has no FOS equivalent. Manual cross-fabric zoning required." Do not allow IVR zone lines to fall through into regular zone parsing.

**Warning signs:**
- Source config contains `ivr enable` or `ivr virtual-domain-add`
- Generated output contains zone names starting with known IVR naming patterns
- Zone member WWNs in output have unexpected VSAN-specific characteristics

**Phase to address:**
Parser implementation phase — add IVR constructs to the skip/warn list before writing zone parsing logic.

---

### Pitfall 11: Device-Alias Name 63/64 Character Limit Discrepancy Across NX-OS Versions

**What goes wrong:**
Prior to NX-OS 9.2(2), device-alias names could be 64 characters. From NX-OS 9.2(2) onward, the limit is 63 characters. A source config from an older switch may contain 64-character device-alias names. If the converter naively passes these through to Brocade (which also allows 64 characters), the Brocade alias is technically valid, but if the source ever upgrades NX-OS, the fabric merge will fail with an ISSU blocker. Additionally, FOS alias names have their own 64-character limit, so a 64-character MDS alias name leaves no room for any name-disambiguation suffix if a sanitization collision occurs.

**Why it happens:**
The limit change between NX-OS versions is not widely known. Developers assume the source's permitted names are always valid on the target.

**How to avoid:**
Enforce a 63-character maximum on all output alias/zone/cfg names (the more conservative limit). If a name must be truncated, emit a warning with original and truncated names. Reserve the last 2 characters for collision-disambiguation suffixes (`_1` through `_9`), meaning the effective usable prefix is 61 characters.

**Warning signs:**
- Source config contains device-alias names of exactly 64 characters
- No truncation warnings appear in converter output despite long names in source

**Phase to address:**
Name sanitization module.

---

### Pitfall 12: Brocade cfgsave vs. cfgenable — Generated Scripts Must Include Both

**What goes wrong:**
`cfgenable` activates a zone configuration in memory and in the effective fabric database. Without `cfgsave`, the configuration is lost on power cycle or reboot. Scripts that only emit `cfgenable` (like Cisco's `zoneset activate`) will produce a working but non-persistent Brocade configuration.

**Why it happens:**
Cisco `zoneset activate` is persistent — it modifies the running-config which is saved with `copy run start`. Brocade separates activation (cfgenable) from persistence (cfgsave). Developers coming from Cisco implicitly assume activation is persistent.

**How to avoid:**
Always emit `cfgsave` as the final command in every generated FOS script, immediately after `cfgenable`. Add a comment before it explaining why: `# cfgsave makes the configuration persistent across reboots`. Emit a warning if for some reason the script omits cfgsave.

**Warning signs:**
- Generated script ends with `cfgenable` with no `cfgsave`
- Ops team reports zoning works until a reboot, then reverts

**Phase to address:**
Output generation phase — make `cfgsave` a mandatory final statement in the script template.

---

### Pitfall 13: Zone Members With Non-WWN Types — Domain/Port, Interface, IP, Symbolic Node Name

**What goes wrong:**
Cisco MDS zones support multiple member types beyond pWWN: `domain-id`, `interface`, `fcid`, `fwwn`, `ip-address`, and `symbolic-nodename`. Brocade FOS zones support WWN members and domain/port pairs. A parser that assumes all zone members are pWWN or device-alias will crash or silently skip non-WWN members. Worse, if domain/port members are partially parsed, the converter might generate syntactically valid but semantically wrong Brocade output (e.g., treating a domain ID as part of a WWN string).

**Why it happens:**
Most documentation shows pWWN-only examples. Non-WWN member types are less common but present in real production configs, especially `interface` members used in QoS scenarios.

**How to avoid:**
Build the parser to recognize all MDS member type keywords explicitly. For unsupported types (interface, fcid, ip-address, symbolic-nodename), emit a per-member warning and skip the member. For domain/port (which has a Brocade equivalent), attempt conversion with a warning noting topology dependency. Never pass unrecognized member types through to output.

**Warning signs:**
- Source config contains `member interface fc1/1` or `member ip-address` lines
- Generated Brocade commands contain non-hexadecimal strings where WWNs are expected

**Phase to address:**
Parser implementation phase — define the full member type enum before parsing begins.

---

### Pitfall 14: Brocade "Semicolon Separator" in Zone Members — Parser Must Handle Wrapped Lines

**What goes wrong:**
Brocade FOS `cfgshow` and `zoneshow` output wraps long member lists across multiple lines with continuation. A parser that reads one line at a time without handling continuation will split a zone's member list into fragments. The first fragment looks like a complete zone; subsequent fragments are unrecognized and silently dropped.

**Why it happens:**
The Brocade CLI wraps output at terminal width. Scripts that capture and parse switch output (as opposed to static config files) encounter this constantly. The semicolon separator within a single logical line makes the boundary between wrapping and intentional newlines ambiguous.

**How to avoid:**
Implement a Brocade parser that treats the member list as a single logical semicolon-delimited string, joining continuation lines before splitting on semicolons. Identify continuation lines by the absence of a command keyword at the start of the line (they typically start with whitespace or a continuation character).

**Warning signs:**
- Parsed alias or zone member count is lower than `alishow`/`zoneshow` output suggests
- Member lists in output stop at an arbitrary short count
- No errors during parsing of a Brocade config known to have large zones

**Phase to address:**
Brocade parser implementation phase.

---

### Pitfall 15: Inactive Zonesets — Zones Defined but Not in Any Active Zoneset Are Still Valid Input

**What goes wrong:**
MDS running-config may contain zone definitions that exist in the full zone database but are not members of the active zoneset. The PROJECT.md notes "VSANs without active zoneset — no zoning to convert," but a VSAN can have an active zoneset with some zones outside it. A converter that only processes zones reachable from the active zoneset will silently omit defined-but-inactive zones, producing an incomplete migration that ops team may not notice until they try to activate a different zoneset on the target.

**Why it happens:**
Converters naturally start from `zoneset activate` to find what to convert. But the source fabric may have a "staging" or "DR" zoneset not currently active.

**How to avoid:**
Parse all zone definitions regardless of active zoneset membership. For zones not in the active zoneset, still generate Brocade output but mark them with a comment `# Not in active zoneset on source`. Emit a summary warning listing how many defined zones were not in the active zoneset. Let the operator decide whether to include them.

**Warning signs:**
- Source `show zoneset active` output has fewer zones than `show zone` output
- Ops team asks "where are the DR zones?" after applying the generated script

**Phase to address:**
Parser implementation phase — separate the "parse all zones" pass from the "identify active zoneset members" pass.

---

## Technical Debt Patterns

| Shortcut | Immediate Benefit | Long-term Cost | When Acceptable |
|----------|-------------------|----------------|-----------------|
| Regex-only NX-OS parser (no AST) | Faster to write | Breaks on whitespace variations, inline comments, continuation lines; hard to extend | Never — start with a proper token-based parser |
| Assume all zone members are pWWN | Simpler code | Silently drops device-alias, fcalias, smart zoning, non-WWN members; corrupt output | Never in production |
| Skip sanitization, pass names through | No renaming warnings | Hyphens silently stripped by old FOS; collisions; 64-char limit exceeded | Never |
| Emit cfgenable without cfgsave | Simpler script | Non-persistent config; reverts on reboot | Never |
| Only process active zoneset | Simpler scoping | Inactive zones silently lost; incomplete migration | Acceptable in v1 if prominently warned |
| Hardcode FOS 8.1.0+ naming rules | Fewer sanitization cases | FABR-1001 errors on mixed-version fabrics | Acceptable if documented and made configurable |

---

## Integration Gotchas

| Integration | Common Mistake | Correct Approach |
|-------------|----------------|------------------|
| NX-OS running-config file | Treating it as a flat list of commands | Parse as a stateful block-structured document with VSAN, zone, fcalias, device-alias sub-contexts |
| Brocade cfgshow/zoneshow output | Treating each output line as a complete record | Join continuation lines before tokenizing; member list is one logical semicolon-delimited string |
| Device-alias database block | Treating it as optional or version-specific | Always parse it; present in all MDS configs using device-alias (standard since 8.5.1) |
| FOS script generation | Assuming cfgenable is sufficient | Always emit defzone + alicreate + zonecreate + cfgcreate + cfgenable + cfgsave in that order |
| Name sanitization | Apply per object in isolation | Build full name inventory first, then sanitize, then detect collisions across full set |

---

## Performance Traps

| Trap | Symptoms | Prevention | When It Breaks |
|------|----------|------------|----------------|
| Resolving device-aliases inside zone parsing loop | O(N*M) lookups on large configs | Build alias lookup map in pass 1; O(1) lookup in pass 2 | Configs with 1000+ aliases and 8000+ zones |
| Re-reading input file for each VSAN | Slow on large configs; multiple file passes | Parse entire file once into an in-memory AST, then walk by VSAN | Configs over ~10MB (rare but possible in large fabrics) |
| Generating FOS commands one-by-one to stdout | Acceptable for normal use | For configs with 5000+ zones, buffer output and write once | Configs exceeding ~8000 zones |

---

## Security Mistakes

| Mistake | Risk | Prevention |
|---------|------|------------|
| Omitting defzone --noaccess from generated script | Target fabric allows all unconfigured devices to communicate | Always prepend defzone --noaccess; make it non-optional |
| Treating conversion warnings as informational only | Ops applies script without reading warnings; silently broken zoning | Emit a warning summary with count at end of output; recommend review before apply |
| Not validating WWN format before emitting | Malformed WWN in cfgshow or device-alias database passed through to Brocade | Validate all WWN strings against `^([0-9a-fA-F]{2}:){7}[0-9a-fA-F]{2}$` pattern before emit |

---

## UX Pitfalls

| Pitfall | User Impact | Better Approach |
|---------|-------------|-----------------|
| Warnings mixed inline with generated commands | Ops cannot separate commands from warnings; cannot pipe output | Write commands to stdout, warnings/errors to stderr with structured prefix (`WARN:`, `ERROR:`, `INFO:`) |
| No summary of what was converted | Ops does not know if conversion was complete | Print a summary table: aliases converted, zones converted, zonesets converted, skipped/warned items |
| Hard stop on first parse error | Ops gets no output from large config due to one edge case | Warn and continue (per PROJECT.md constraint); output best-effort result with full warning log |
| Generated FOS script not idempotent | Re-running script on Brocade causes "object already exists" errors | Either prefix each command with a delete-if-exists guard, or document that the script requires a clean target namespace |

---

## "Looks Done But Isn't" Checklist

- [ ] **Device-alias enhanced mode:** Parser handles `member device-alias <name>` in zones — not just `member pwwn`. Verify with a test config that was captured from an NX-OS 8.5+ switch.
- [ ] **fcalias support:** Parser locates VSAN-scoped `fcalias name <X> vsan <N>` blocks and resolves them. Verify with a config that uses fcalias (not device-alias).
- [ ] **Smart zoning keywords:** Parser strips `init`/`target`/`both` from member lines without corrupting the WWN. Verify with a config from a smart-zoning-enabled VSAN.
- [ ] **IVR zones skipped:** Parser does not include `ivr zone` entries in regular zone output. Verify with a config containing `ivr enable`.
- [ ] **Name sanitization:** Output contains no hyphens, dollar signs, or carets when targeting default FOS mode. Verify by running generated script through a name validator.
- [ ] **Collision detection:** When two sanitized names collide, both are present in output with disambiguated suffixes. Verify by constructing two source names that sanitize to the same string.
- [ ] **cfgsave present:** Every generated FOS script ends with `cfgsave`. Verify with a script generation test.
- [ ] **defzone --noaccess present:** Every generated FOS script starts with `defzone --noaccess`. Verify with a script generation test.
- [ ] **Multi-VSAN warning:** Tool emits a warning when source config contains more than one VSAN. Verify with a multi-VSAN config.
- [ ] **Non-WWN members warned and skipped:** Tool emits a warning for interface, ip-address, fcid, and symbolic-nodename members. Verify with a config containing each type.

---

## Recovery Strategies

| Pitfall | Recovery Cost | Recovery Steps |
|---------|---------------|----------------|
| Silent alias name corruption (hyphen stripped) | HIGH | Re-run conversion with sanitization enabled; diff new output against applied config; update zone membership on live Brocade switch |
| Name collision overwrote wrong alias | HIGH | Re-run conversion; review collision warnings; manually identify which device lost its alias; re-zone affected devices |
| cfgsave omitted; config lost on reboot | MEDIUM | Re-apply generated script; if script was discarded, re-run converter from source config |
| IVR zones partially converted | MEDIUM | Identify IVR zones in warning log; manually create equivalent cross-fabric zoning on Brocade |
| defzone --allaccess left default | HIGH | Run `defzone --noaccess` on Brocade switch immediately; audit any unexpected device communication that occurred |
| Multi-VSAN merge produced wrong zone set | HIGH | Re-run with per-VSAN output mode; manually review and apply per-VSAN |

---

## Pitfall-to-Phase Mapping

| Pitfall | Prevention Phase | Verification |
|---------|------------------|--------------|
| Device-alias enhanced mode parsing | Parser implementation — zone member types | Test with enhanced-mode fixture config |
| Brocade hyphen silent stripping | Name sanitization module | Unit test: names with hyphens produce underscores in output |
| FOS enhanced naming fabric-wide requirement | Name sanitization + CLI flags | Integration test with conservative character set as default |
| fcalias vs. device-alias coexistence | Parser implementation — alias resolution | Test with config containing both alias types |
| Name collision after sanitization | Name sanitization module | Unit test: two names that sanitize identically produce distinct output names |
| Multi-VSAN VSAN-scoped parsing | Parser design — AST and data model | Test with 3-VSAN config; verify zone VSAN scoping is preserved |
| Smart zoning keywords | Parser implementation | Test fixture with `init`/`target`/`both` keywords |
| Brocade spaces ignored in zone names | Brocade parser implementation | Test: `Zone 1` and `Zone1` treated as same object |
| Default zone policy mismatch | Output generation / script template | Test: every generated script contains `defzone --noaccess` |
| IVR zones misidentified as regular zones | Parser implementation | Test fixture with IVR config; verify no ivr-zone output |
| 63/64-char limit discrepancy | Name sanitization module | Unit test: 64-char name truncated to 63 |
| cfgsave omitted from script | Output generation / script template | Test: every generated script ends with `cfgsave` |
| Non-WWN zone member types | Parser implementation | Test fixture with interface, ip-address, fcid members |
| Brocade wrapped member list | Brocade parser implementation | Test: simulate terminal-wrapped cfgshow output |
| Inactive zones silently omitted | Parser implementation — zone scope | Test: source config with zones not in active zoneset |

---

## Sources

- Cisco MDS 9000 Series Fabric Configuration Guide, Release 9.x — Distributing Device Alias Services: https://www.cisco.com/c/en/us/td/docs/dcn/mds9000/sw/9x/configuration/fabric/cisco-mds-9000-nx-os-fabric-configuration-guide-9x/distributing_device_alias_services.html
- Cisco MDS 9000 Series Fabric Configuration Guide, Release 9.x — Configuring and Managing Zones: https://www.cisco.com/c/en/us/td/docs/dcn/mds9000/sw/9x/configuration/fabric/cisco-mds-9000-nx-os-fabric-configuration-guide-9x/configuring_and_managing_zones.html
- Broadcom FOS 9.2.x zoneCreate command reference: https://techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-commands/9-2-x/Fabric-OS-Commands/zoneCreate.html
- Broadcom FOS 9.2.x defZone command reference: https://techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-commands/9-2-x/Fabric-OS-Commands/defZone.html
- Broadcom FOS 9.2.x Zone Configurations administration guide: https://techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-administration/9-2-x/Administering-Advanced-Zoning-AG/v26770788.html
- Broadcom FOS 9.2.x Setting the Default Zoning Mode: https://techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-administration/9-2-x/Administering-Advanced-Zoning-AG/v26772700.html
- Dell KB 000227366 — Enhanced Zone Object naming feature not supported: https://www.dell.com/support/kbdoc/en-us/000227366/connectrix-brocade-unable-to-create-alias-zones-due-to-error-enhanced-zone-object-naming-feature-is-not-supported-by-the-fabric
- Cisco Smart Zoning technical note: https://www.cisco.com/c/en/us/support/docs/storage-networking/zoning/116390-technote-smartzoning-00.html
- Cisco MDS NX-OS Release 9.2(2) Release Notes — device-alias 64 char ISSU blocker: https://www.cisco.com/c/en/us/td/docs/dcn/mds9000/sw/9x/release-notes/cisco-mds-9000-nx-os-release-notes-922.html
- GitHub Cisco-SAN/ZoneMigrator — documented limitations (pwwn-only, Windows-only, FOS 7.x/8.x only): https://github.com/Cisco-SAN/ZoneMigrator
- PenguinPunk.net — Brocade alias and zone syntax hyphen-stripping gotcha: https://www.penguinpunk.net/blog/brocade-alias-and-zone-syntax-or-how-fos-is-a-love-hate-thing/
- Broadcom SAN Scalability Guidelines for Fabric OS 9.x: https://docs.broadcom.com/doc/SAN-Scalability-FOS9x-UG
- Nick Tailor — Cisco vs Brocade SAN Switch Commands: https://nicktailor.com/tech-blog/cisco-vs-brocade-san-switch-commands-explained-with-diagnostics-and-examples/
- Dell KB on Brocade cfgsave and cfgenable: https://storagearea.network/how-to-save-brocade-zoning-configuration-cfgsave/
- Cisco Inter-VSAN Routing Configuration Guide, Release 9.x: https://www.cisco.com/c/en/us/td/docs/dcn/mds9000/sw/9x/configuration/ivr/cisco-mds-9000-nx-os-ivr-configuration-guide-9x/basic_ivr.html

---
*Pitfalls research for: SAN zoning config conversion (Cisco MDS NX-OS to/from Brocade FOS)*
*Researched: 2026-03-28*
