# Phase 6: MDS Emitter - Research

**Researched:** 2026-03-29
**Domain:** Go io.Writer pattern, Cisco NX-OS CLI command syntax, symmetric emitter design
**Confidence:** HIGH

---

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONV-04 | Convert Brocade `alias:` definitions to MDS `device-alias` entries | IR `Alias.Name` and `Alias.PWWN` carry the data; emit one `device-alias name X pwwn Y` line per entry inside a `device-alias database` / `device-alias commit` block |
| CONV-05 | Convert Brocade `zone:` definitions to MDS `zone name X vsan 1` definitions | IR `Zone.Name` and `Zone.Members` carry the data; Brocade IR uses VSAN 0 as sentinel — emit all zones as `vsan 1` (the conventional MDS VSAN for a converted Brocade fabric) |
| CONV-06 | Convert Brocade `cfg:` definitions to MDS `zoneset name X vsan 1` definitions | IR `ZoneConfig.Name` and `ZoneConfig.ZoneNames` carry the data; emit `zoneset name X vsan 1` block followed by `zoneset activate name X vsan 1` |
| OUT-03 | Write MDS NX-OS config commands for the brocade2mds direction (device-alias, zone, zoneset, zoneset activate) | Emitter produces a complete, paste-ready NX-OS config fragment in the canonical order: device-alias database block, zone blocks, zoneset block, zoneset activate |

</phase_requirements>

---

## Summary

Phase 6 implements `internal/emitter/mds/emitter.go`, the symmetric counterpart of the Brocade emitter built in Phase 5. The package stub (`internal/emitter/mds/doc.go`) already exists. The emitter consumes a sanitized `*ir.ZoningConfig` (produced by the Brocade parser and sanitizer) and writes correct NX-OS CLI commands to an `io.Writer`.

The NX-OS zoning configuration format is block-structured: a `device-alias database` block groups all alias definitions, followed by per-zone blocks, followed by a zoneset block. The output must be a complete, paste-ready config fragment — not individual commands issued interactively, but the full declarative config syntax that NX-OS accepts via `copy paste` into the EXEC prompt.

The design is a near-perfect mirror of the Brocade emitter: same function signature shape (`Emit(*ir.ZoningConfig, io.Writer) error`), same `sort.Strings` determinism pattern, same `cfg.Warnings` append-only pattern for non-fatal issues, same `io.Writer` abstraction. The primary difference is that this emitter has no `scriptMode` parameter — MDS config output is a single format (there is no separate "script" vs "commands-only" distinction in NX-OS paste config).

A key design decision is how to handle the VSAN. Brocade IR uses VSAN 0 as a sentinel for "single fabric, no VSAN concept." When emitting MDS output, the emitter must pick a VSAN number. The canonical choice is VSAN 1 — this is the default VSAN in NX-OS and the most common target for a migrated Brocade fabric. This choice should be documented clearly in code comments and in the plan, since ops teams will need to know to adjust the VSAN number if their target fabric uses a different VSAN.

The TDD pattern established in phases 2-5 applies: Plan 01 writes failing tests (`emitter_test.go`), Plan 02 implements the emitter making all tests pass.

**Primary recommendation:** Implement `Emit(*ir.ZoningConfig, io.Writer) error` using `fmt.Fprintf` for line-by-line emission. Sort map keys for deterministic output. Use VSAN 1 as the target VSAN for all Brocade-sourced IR. No new dependencies required.

---

## Project Constraints (from CLAUDE.md)

| Constraint | Source | Enforcement |
|------------|--------|-------------|
| Single Go binary, no runtime deps | CLAUDE.md Tech Stack | No new external packages; `fmt`, `io`, `sort`, `strings` stdlib only |
| Warn and continue — partial output better than stopping | CLAUDE.md Constraints | Emitter appends to cfg.Warnings on skipped members; never returns error for non-fatal issues |
| Write all output to `io.Writer` interface | CLAUDE.md Stack Patterns | `Emit(cfg *ir.ZoningConfig, w io.Writer) error` — never writes directly to os.Stdout |
| `ZoningConfig` is canonical intermediate representation | CLAUDE.md Stack Patterns | Emitter receives `*ir.ZoningConfig`, does not parse any input |
| Use `require` (not `assert`) in tests | CLAUDE.md Supporting Libraries | All test assertions use `require.Equal`, `require.Contains` |
| Table-driven tests | CLAUDE.md Stack Patterns | Emitter tests use `tests []struct { name, ... }` pattern |
| No `html/template` | CLAUDE.md What NOT to Use | Must use `text/template` if templates are used; however `fmt.Fprintf` is simpler for this emitter |
| IR package has zero imports | CLAUDE.md Decisions | Emitter imports `internal/ir`; IR package must NOT import `internal/emitter` |
| No logrus | CLAUDE.md What NOT to Use | Warnings go to `cfg.Warnings []string`; never to a logger |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `fmt` | Go 1.26.1 (go.mod) | Line-by-line command emission via `fmt.Fprintf` | Simpler and cleaner than `text/template` for fixed-format single-line NX-OS commands |
| stdlib `io` | Go 1.26.1 | `io.Writer` abstraction for output destination | Established project pattern; makes emitter testable via `bytes.Buffer` |
| stdlib `sort` | Go 1.26.1 | Deterministic map iteration order | Go maps are unordered; sorted keys guarantee reproducible output |
| stdlib `strings` | Go 1.26.1 | Member joining if needed; minor string ops | Consistent with Brocade emitter stdlib-only approach |
| `github.com/stretchr/testify/require` | v1.11.1 (go.mod) | Test assertions | Already in project; `require` sub-package per CLAUDE.md |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `bytes` | Go 1.26.1 | `bytes.Buffer` as `io.Writer` in tests | Capture emitter output for assertion |

### No New Dependencies Required

All emitter functionality is achievable with stdlib. Existing `go.mod` has everything needed. The Phase 5 Brocade emitter used only `fmt`, `io`, `sort`, `strings` — Phase 6 follows the same pattern.

### Installation

No install step needed. All required packages are stdlib or already in `go.mod`.

---

## Architecture Patterns

### Recommended Project Structure

The emitter lives in the already-created stub package:

```
internal/
└── emitter/
    ├── brocade/         # Phase 5 — complete
    │   ├── doc.go
    │   ├── emitter.go
    │   └── emitter_test.go
    └── mds/             # Phase 6 — this phase
        ├── doc.go       # EXISTS — package stub with comment
        ├── emitter.go   # TO CREATE — Plan 02
        └── emitter_test.go  # TO CREATE — Plan 01
```

### Pattern 1: NX-OS Config Block Structure

NX-OS zoning config uses a block-structured format. The correct paste order is:

```
device-alias database
  device-alias name <alias-name> pwwn <pwwn>
  device-alias name <alias-name> pwwn <pwwn>
  ...
device-alias commit

zone name <zone-name> vsan <vsan>
  member device-alias <alias-name>
  member pwwn <pwwn>
  ...

zone name <zone-name> vsan <vsan>
  ...

zoneset name <zoneset-name> vsan <vsan>
  member <zone-name>
  ...

zoneset activate name <zoneset-name> vsan <vsan>
```

**Key formatting rules (from testdata/mds/basic.cfg and real NX-OS configs):**
- `device-alias database` — no arguments, opens the block
- `  device-alias name X pwwn Y` — 2-space indent inside the database block
- `device-alias commit` — closes the block, no indent
- `zone name X vsan N` — no indent on the zone declaration line
- `  member device-alias X` or `  member pwwn X` — 2-space indent for each member
- `zoneset name X vsan N` — no indent
- `  member X` — 2-space indent for zone membership
- `zoneset activate name X vsan N` — standalone line, no indent, no block

### Pattern 2: VSAN 0 Sentinel Handling

Brocade IR uses `Zone.VSAN = 0` as a sentinel meaning "no VSAN (single fabric)." When emitting MDS output, the emitter must emit a concrete VSAN. The established project convention is:

- **All Brocade-to-MDS zones use VSAN 1** (the NX-OS default VSAN)
- The emitter should define a package-level constant `const defaultVSAN = 1`
- The zone.VSAN field check: if `zone.VSAN == 0` (Brocade sentinel), emit VSAN 1; if `zone.VSAN > 0` (already an MDS VSAN), use it directly

This mirrors the Brocade emitter's handling of composite map keys — the emitter uses `zone.Name` not the map key, and here uses a real VSAN number not the 0 sentinel.

### Pattern 3: Emit() Function Signature

The Phase 5 precedent establishes the function signature shape. Phase 6 omits `scriptMode` because NX-OS has a single paste-config format:

```go
// Source: mirrors internal/emitter/brocade/emitter.go signature
func Emit(cfg *ir.ZoningConfig, w io.Writer) error
```

No `scriptMode` parameter: NX-OS paste-config format is the only output format needed for Phase 6. (The OUT-02 equivalent — "script with preamble/postamble" — is not a Phase 6 requirement.)

### Pattern 4: Zone Member Type Resolution

Brocade IR members can be type `"alias"`, `"pwwn"`, or `"unsupported"`. The MDS emitter must:

- `"alias"` member → emit as `  member device-alias <value>`
- `"pwwn"` member → emit as `  member pwwn <value>`
- `"unsupported"` member → skip with warning appended to `cfg.Warnings` (same warn-and-continue pattern as Brocade emitter)

A zone with all members unsupported should be skipped entirely (matching the Brocade emitter's behavior), with a warning emitted and the zone excluded from its zoneset's member list.

### Pattern 5: ZoneConfig Map Key vs. Zone Name

The established decision from Phase 5 applies here: always use `zone.Name` (the struct field), not the map key (which may be `name@vsanN` for MDS-sourced IR). For Brocade-sourced IR, map keys are plain names — but using `zone.Name` is the invariant that must be maintained for correctness.

### Anti-Patterns to Avoid

- **Emitting VSAN 0 literally:** `zone name X vsan 0` is invalid NX-OS syntax. Always resolve the sentinel to a real VSAN number.
- **Using map key as output value:** Map keys for MDS-sourced IR use `name@vsanN` composite format. Always use `zone.Name` field, never the map key.
- **Adding scriptMode parameter:** OUT-03 does not require a script wrapper. Do not add a scriptMode parameter — YAGNI, and it would be inconsistent with the requirements.
- **Using `html/template`:** Would corrupt pWWN values by escaping colons. Use `fmt.Fprintf` or `text/template` (the non-HTML variant).
- **Emitting `zoneset activate` as a comment:** Unlike Brocade's `cfgenable` which is always commented, NX-OS `zoneset activate` is a real config command that must be emitted as-is. The emitter must include it.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Map iteration order | Custom sort logic | `sort.Strings(keys)` then iterate | Go generics helper `sortedStringKeys[V any]` already proven in brocade emitter — replicate verbatim |
| pWWN format validation | Custom regex in emitter | Accept whatever IR contains | Sanitizer (Phase 4) already validated/cleaned names; emitter trusts the IR |
| Output buffering | Custom write buffering | `io.Writer` passed in from caller | Caller controls buffering; emitter writes directly to w |

**Key insight:** The MDS emitter is a 100-line function. All complexity is in the data structures (already defined in IR), not in the emission logic. Do not add abstraction layers.

---

## Common Pitfalls

### Pitfall 1: VSAN 0 Leaking Into Output

**What goes wrong:** `zone name Host-Zone vsan 0` appears in emitted output — this is syntactically invalid in NX-OS and would cause a parse error on the switch.

**Why it happens:** Brocade IR carries `Zone.VSAN = 0` as a sentinel. Passing it directly to `fmt.Fprintf` without resolution emits the literal 0.

**How to avoid:** Define `const defaultVSAN = 1`. In the zone emission loop: `vsan := zone.VSAN; if vsan == 0 { vsan = defaultVSAN }`. Assert in tests that `vsan 0` never appears in output.

**Warning signs:** Test output contains `vsan 0`.

### Pitfall 2: Missing `device-alias commit`

**What goes wrong:** The `device-alias database` block is opened but never closed with `device-alias commit`. NX-OS will accept the config but the device-alias database will not be committed to the running config on older NX-OS versions.

**Why it happens:** Focusing on the `device-alias name` lines without considering the block structure.

**How to avoid:** Always emit `device-alias commit` immediately after the last `device-alias name` line in the block. Test for the exact string `device-alias commit` in the output.

**Warning signs:** Output has `device-alias database` but no `device-alias commit`.

### Pitfall 3: Forgetting `zoneset activate`

**What goes wrong:** The `zoneset name X vsan 1` block is emitted but `zoneset activate name X vsan 1` is not. The zoneset exists in config but is not active on the switch.

**Why it happens:** Treating the activate as an optional postamble (like `cfgenable` in Brocade) when it is actually a mandatory config command.

**How to avoid:** For every emitted zoneset, always follow with `zoneset activate name X vsan N`. Test for the presence of `zoneset activate name X vsan 1` in output. Unlike Brocade's `cfgenable`, this is never commented out.

**Warning signs:** Output has `zoneset name` but no `zoneset activate name`.

### Pitfall 4: Composite Map Key Leaking Into Output

**What goes wrong:** Zone name `Zone-A@vsan10` appears in the zoneset's `member` line instead of `Zone-A`.

**Why it happens:** Iterating over map keys and using the key as the zone name, rather than using `zone.Name`.

**How to avoid:** All emission uses `zone.Name` field, never the map key. Mirror exactly how Phase 5's `Emit()` handles this.

**Warning signs:** Output contains `@vsan` in any command.

### Pitfall 5: Output Order for Multiple ZoneConfigs

**What goes wrong:** Multiple zonesets are emitted in non-deterministic order, making output unstable across runs.

**Why it happens:** Iterating `cfg.ZoneConfigs` map without sorting keys.

**How to avoid:** Use `sortedStringKeys(cfg.ZoneConfigs)` before the ZoneConfig loop, exactly as the Brocade emitter does. Add a determinism test (repeated Emit calls produce identical output).

---

## Code Examples

### NX-OS Device-Alias Block Structure

```
device-alias database
  device-alias name host_01 pwwn 10:00:00:00:c9:ab:cd:ef
  device-alias name storage_01 pwwn 50:05:07:61:01:23:45:67
device-alias commit
```

Source: verified against `testdata/mds/basic.cfg` in this repository.

### NX-OS Zone Block Structure

```
zone name fabric_zone1 vsan 1
  member device-alias host_01
  member pwwn 50:05:07:61:01:23:45:67

zone name fabric_zone2 vsan 1
  member pwwn 10:00:00:00:c9:ab:cd:ef
```

Source: verified against `testdata/mds/basic.cfg` in this repository.

### NX-OS Zoneset Block with Activate

```
zoneset name Production_cfg vsan 1
  member fabric_zone1
  member fabric_zone2

zoneset activate name Production_cfg vsan 1
```

Source: verified against `testdata/mds/basic.cfg` and `testdata/mds/multi_vsan.cfg` in this repository.

### Emit() Skeleton

```go
// Source: mirrors internal/emitter/brocade/emitter.go structure
func Emit(cfg *ir.ZoningConfig, w io.Writer) error {
    const defaultVSAN = 1

    // --- Aliases (CONV-04) ---
    aliasKeys := sortedStringKeys(cfg.Aliases)
    if len(aliasKeys) > 0 {
        fmt.Fprintln(w, "device-alias database")
        for _, key := range aliasKeys {
            alias := cfg.Aliases[key]
            fmt.Fprintf(w, "  device-alias name %s pwwn %s\n", alias.Name, alias.PWWN)
        }
        fmt.Fprintln(w, "device-alias commit")
        fmt.Fprintln(w)
    }

    // --- Zones (CONV-05) ---
    emittedZones := make(map[string]bool)
    zoneKeys := sortedStringKeys(cfg.Zones)
    if len(zoneKeys) > 0 {
        for _, key := range zoneKeys {
            zone := cfg.Zones[key]
            vsan := zone.VSAN
            if vsan == 0 {
                vsan = defaultVSAN
            }
            // filter unsupported members, warn-and-continue
            var hasValid bool
            for _, m := range zone.Members {
                if m.Type != "unsupported" {
                    hasValid = true
                }
            }
            if !hasValid {
                cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
                    "zone %q has no valid NX-OS members after filtering unsupported types — skipped",
                    zone.Name,
                ))
                continue
            }
            fmt.Fprintf(w, "zone name %s vsan %d\n", zone.Name, vsan)
            for _, m := range zone.Members {
                switch m.Type {
                case "alias":
                    fmt.Fprintf(w, "  member device-alias %s\n", m.Value)
                case "pwwn":
                    fmt.Fprintf(w, "  member pwwn %s\n", m.Value)
                // "unsupported" skipped silently (already warned at parse time)
                }
            }
            fmt.Fprintln(w)
            emittedZones[zone.Name] = true
        }
    }

    // --- ZoneConfigs / Zonesets (CONV-06, OUT-03) ---
    cfgKeys := sortedStringKeys(cfg.ZoneConfigs)
    for _, key := range cfgKeys {
        zc := cfg.ZoneConfigs[key]
        vsan := zc.VSAN
        if vsan == 0 {
            vsan = defaultVSAN
        }
        var filteredZoneNames []string
        for _, zoneName := range zc.ZoneNames {
            if emittedZones[zoneName] {
                filteredZoneNames = append(filteredZoneNames, zoneName)
            }
        }
        if len(filteredZoneNames) == 0 {
            continue
        }
        fmt.Fprintf(w, "zoneset name %s vsan %d\n", zc.Name, vsan)
        for _, zoneName := range filteredZoneNames {
            fmt.Fprintf(w, "  member %s\n", zoneName)
        }
        fmt.Fprintln(w)
        fmt.Fprintf(w, "zoneset activate name %s vsan %d\n", zc.Name, vsan)
        fmt.Fprintln(w)
    }

    return nil
}

func sortedStringKeys[V any](m map[string]V) []string {
    keys := make([]string, 0, len(m))
    for k := range m {
        keys = append(keys, k)
    }
    sort.Strings(keys)
    return keys
}
```

Note: `sortedStringKeys` is a generic helper identical to the one in the Brocade emitter. Since they are in different packages, each package defines its own unexported copy — no shared utility package is needed.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate `alicreate`/`zonecreate` commands for Brocade | `device-alias`/`zone name` block syntax for NX-OS | N/A — format-specific | MDS config uses block-declarative format; Brocade uses imperative create commands |
| `cfgenable <cfg>` commented in Brocade script | `zoneset activate name X vsan N` NOT commented in MDS output | N/A — format-specific | Activate is a real config statement in NX-OS, not a post-activation command |

---

## Open Questions

1. **VSAN selection for converted config**
   - What we know: Brocade IR carries VSAN 0 as a sentinel. The emitter must pick a concrete VSAN.
   - What's unclear: Which VSAN number the ops team's target MDS fabric uses. VSAN 1 is standard default but real deployments often use a dedicated VSAN (e.g., 100, 200).
   - Recommendation: Hard-code VSAN 1 as the Phase 6 default with a prominent code comment. Phase 7 (CLI Wiring) can expose a `--target-vsan` flag if needed. Do not block Phase 6 on this decision.

2. **Handling zones with mixed alias and pwwn members from Brocade IR**
   - What we know: The Brocade parser produces IR with members typed as `"alias"` (by name) or `"pwwn"` (direct pWWN). Both are valid in NX-OS zone members.
   - What's unclear: Whether ops teams prefer alias expansion (emit `member pwwn <resolved-pWWN>` even when the IR has an alias member) vs. preserving alias references.
   - Recommendation: Preserve alias references as `member device-alias X`. The IR alias map already carries the name→pWWN mapping; if resolution is needed it can be done in Phase 7 or as a flag.

---

## Environment Availability

Step 2.6: SKIPPED — Phase 6 is purely code/config changes. No external tools, services, or CLIs beyond the Go toolchain (already verified as available in prior phases).

---

## Sources

### Primary (HIGH confidence)

- `internal/ir/zoningconfig.go` — IR struct definitions, VSAN sentinel convention
- `internal/emitter/brocade/emitter.go` — Established emitter pattern; Phase 6 mirrors this exactly
- `internal/emitter/brocade/emitter_test.go` — Test structure and table-driven pattern to replicate
- `testdata/mds/basic.cfg` — Verified NX-OS zone config format (device-alias, zone, zoneset, activate)
- `testdata/mds/multi_vsan.cfg` — Multi-VSAN NX-OS format; establishes `zoneset activate` is a mandatory non-commented line
- `testdata/mds/edge_cases.cfg` — Edge cases including empty zone, IVR zone (irrelevant for emitter), orphan zone

### Secondary (MEDIUM confidence)

- `.planning/STATE.md` Accumulated Decisions — VSAN 0 sentinel convention, map key vs. zone.Name rule, warn-and-continue pattern
- `.planning/phases/05-brocade-emitter/05-RESEARCH.md` — Phase 5 research establishes the emitter architecture that Phase 6 replicates

### Tertiary (LOW confidence)

- None. All findings are verified against in-repository source files.

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — all packages are stdlib or already in go.mod; no new libraries needed
- Architecture: HIGH — NX-OS format verified against in-repo fixtures; emitter pattern proven in Phase 5
- Pitfalls: HIGH — VSAN 0 sentinel, `device-alias commit`, `zoneset activate` requirements all verified against testdata fixtures

**Research date:** 2026-03-29
**Valid until:** 2026-06-29 (stable — NX-OS CLI format and Go stdlib do not change at this cadence)
