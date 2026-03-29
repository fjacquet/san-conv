# Phase 5: Brocade Emitter - Research

**Researched:** 2026-03-29
**Domain:** Go text/template output generation, Brocade FOS CLI command syntax, io.Writer pattern
**Confidence:** HIGH

---

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| CONV-01 | Convert MDS device-alias and fcalias entries to Brocade `alicreate` commands with pWWN members | IR carries sanitized `Alias.Name` and `Alias.PWWN`; emit one `alicreate "<name>", "<pwwn>"` per Alias entry |
| CONV-02 | Convert MDS zone definitions to Brocade `zonecreate` commands, resolving alias/device-alias references | IR `ZoneMember.Type` is "alias" or "pwwn"; emit alias members by name, pwwn members by value; semicolon-separated member list |
| CONV-03 | Convert MDS zoneset definitions to Brocade `cfgcreate` commands | IR `ZoneConfig.ZoneNames` already updated by sanitizer; emit one `cfgcreate "<name>", "<z1;z2;...>"` per ZoneConfig |
| OUT-01 | Write Brocade FOS CLI commands to stdout (or `--output` file) in correct application order: alicreate → zonecreate → cfgcreate | Emitter must process all Aliases first, then Zones, then ZoneConfigs — not interleaved |
| OUT-02 | Generate executable shell script with `defzone --noaccess` preamble, all zone commands, commented `cfgenable`, and `cfgsave` postamble | Script mode wraps the OUT-01 output between mandatory preamble/postamble; `cfgenable` always commented with explanation |

</phase_requirements>

---

## Summary

Phase 5 implements `internal/emitter/brocade/emitter.go`, consuming a sanitized `*ir.ZoningConfig` and writing correct FOS CLI commands to an `io.Writer`. The emitter lives in the package stub already declared in `internal/emitter/brocade/doc.go` — the package comment there already specifies the expected signature and behavior.

The emitter has two distinct output modes. The **commands-only mode** (OUT-01) writes `alicreate`, `zonecreate`, and `cfgcreate` lines in guaranteed order. The **script mode** (OUT-02) wraps the same commands between a `defzone --noaccess` preamble and a `cfgsave` postamble, with `cfgenable` emitted as a commented line (never executable). Both modes write to the same `io.Writer` abstraction per the CLAUDE.md pattern.

The critical design decisions are already locked by the project's accumulated decisions in STATE.md: (1) `defzone --noaccess` and `cfgsave` are mandatory and non-omittable; (2) `cfgenable` is always commented out; (3) the emitter accepts an `io.Writer` so CLI wiring (Phase 7) controls where output goes. The emitter must not know about files, stdout, or CLI flags — that is Phase 7's concern.

The established TDD pattern (red phase fixtures + test file, then green phase implementation) applies here. Two plans: Plan 01 writes `emitter_test.go` with failing tests that define the complete behavioral contract; Plan 02 implements `emitter.go` making all tests pass.

**Primary recommendation:** Implement `Emit(*ir.ZoningConfig, io.Writer, bool) error` where the bool signals script mode. Use `fmt.Fprintf` for per-line output (no template needed — the command format is simple enough). Sort map keys for deterministic output order.

---

## Project Constraints (from CLAUDE.md)

| Constraint | Source | Enforcement |
|------------|--------|-------------|
| Single Go binary, no runtime deps | CLAUDE.md Tech Stack | No new external packages; `fmt`, `io`, `sort`, `strings` stdlib only |
| Warn and continue — partial output better than stopping | CLAUDE.md Constraints | Emitter appends to cfg.Warnings on skipped members; never returns error for non-fatal issues |
| Write all output to `io.Writer` interface | CLAUDE.md Stack Patterns | `Emit(cfg, w io.Writer, scriptMode bool)` — never writes directly to os.Stdout |
| `ZoningConfig` is canonical intermediate representation | CLAUDE.md Stack Patterns | Emitter receives `*ir.ZoningConfig`, does not parse any input |
| Use `require` (not `assert`) in tests | CLAUDE.md Supporting Libraries | All test assertions use `require.Equal`, `require.Contains` |
| Table-driven tests | CLAUDE.md Stack Patterns | Emitter tests use `tests []struct { name, ... }` pattern |
| No `html/template` | CLAUDE.md What NOT to Use | Must use `text/template` (if templates used at all) — `html/template` would corrupt pWWN colons |
| IR package has zero imports | CLAUDE.md Decisions | Emitter imports `internal/ir`; IR package must not import `internal/emitter` |
| `defzone --noaccess` and `cfgsave` mandatory, `cfgenable` always commented | STATE.md Decisions | Output mode flag cannot suppress these — they are unconditional |
| No logrus | CLAUDE.md What NOT to Use | Warnings go to `cfg.Warnings []string` |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `fmt` | Go 1.26.1 (go.mod) | Line-by-line command emission via `fmt.Fprintf` | Simpler than `text/template` for fixed-format single-line commands; zero allocation concern for a CLI tool |
| stdlib `io` | Go 1.26.1 | `io.Writer` abstraction for output destination | Already used in MDS parser; makes emitter testable via `strings.Builder` or `bytes.Buffer` in tests |
| stdlib `sort` | Go 1.26.1 | Deterministic map iteration order (aliases, zones, configs) | Go maps are unordered; sorted keys guarantee reproducible output for ops team |
| stdlib `strings` | Go 1.26.1 | Member list joining with `;` separator | `strings.Join(members, ";")` for zonecreate/cfgcreate member lists |
| `github.com/stretchr/testify/require` | v1.11.1 (go.mod) | Test assertions | Already in project; `require` sub-package per CLAUDE.md |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `text/template` | Go 1.26.1 | Alternative to `fmt.Fprintf` for multi-line output blocks | Use only if the preamble/postamble block grows beyond 3-4 lines — for current scope, `fmt.Fprintf` is cleaner |
| stdlib `bytes` | Go 1.26.1 | `bytes.Buffer` as `io.Writer` in tests | Capture emitter output for assertion without touching real files |

### No New Dependencies Required

All emitter functionality is achievable with stdlib. Existing `go.mod` has everything needed.

### Alternatives Considered

| Recommended | Alternative | Tradeoff |
|-------------|-------------|----------|
| `fmt.Fprintf` per command line | `text/template` with full template | Template adds indirection and a compile-time string for simple fixed-format commands. `fmt.Fprintf` is transparent and lint-clean. Choose template only if output format becomes complex (multiple variants). |
| `sort.Strings` on map keys | Range maps directly | Maps are unordered in Go. Direct range produces non-deterministic output — unacceptable for ops tooling where diff visibility matters. |
| Single `Emit` with scriptMode bool | Two separate functions `EmitCommands` / `EmitScript` | Two functions duplicate the core loop. Single function with a bool is standard for modes with 80% shared code. |

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
└── emitter/
    └── brocade/
        ├── doc.go           # Already exists — package declaration
        ├── emitter.go       # New: Emit(cfg, w, scriptMode) function + helpers
        └── emitter_test.go  # New: table-driven tests (TDD red phase in Plan 01)
```

### Pattern 1: io.Writer Emitter (established by CLAUDE.md)

**What:** Emitter accepts `io.Writer` — never imports `os` or references stdout/files directly.

**When to use:** Always. This is a hard CLAUDE.md constraint.

**Example:**
```go
// Source: CLAUDE.md Stack Patterns
func Emit(cfg *ir.ZoningConfig, w io.Writer, scriptMode bool) error {
    if scriptMode {
        fmt.Fprintln(w, "defzone --noaccess")
    }
    // ... emit alicreate, zonecreate, cfgcreate in order ...
    if scriptMode {
        fmt.Fprintf(w, "# cfgenable \"%s\"  # Uncomment and run manually after verifying the config\n", cfgName)
        fmt.Fprintln(w, "cfgsave")
    }
    return nil
}
```

### Pattern 2: Sorted Map Keys for Deterministic Output (established in parsers)

**What:** Extract keys from IR maps, sort them, then range the sorted slice.

**When to use:** Every time an IR map is iterated for output.

**Example:**
```go
// Source: established Go idiom — consistent with project sort usage in sanitizer
aliasNames := make([]string, 0, len(cfg.Aliases))
for name := range cfg.Aliases {
    aliasNames = append(aliasNames, name)
}
sort.Strings(aliasNames)
for _, name := range aliasNames {
    alias := cfg.Aliases[name]
    fmt.Fprintf(w, "alicreate %q, %q\n", alias.Name, alias.PWWN)
}
```

### Pattern 3: Member List Construction (semicolon-separated)

**What:** Zone members (aliases or pWWNs) and cfg zone names are joined with `;` separator — no trailing semicolon.

**When to use:** `zonecreate` member list, `cfgcreate` zone list.

**Example:**
```go
// Source: verified from techdocs.broadcom.com aliCreate and zoneCreate pages
members := make([]string, 0, len(zone.Members))
for _, m := range zone.Members {
    if m.Type == "unsupported" {
        continue // skip with warn-and-continue
    }
    members = append(members, m.Value) // alias name OR pWWN value
}
fmt.Fprintf(w, "zonecreate %q, %q\n", zone.Name, strings.Join(members, ";"))
```

### Pattern 4: cfgenable as Commented Line

**What:** `cfgenable` must never be an executable statement in generated scripts. It appears as a shell comment with an explanatory message.

**When to use:** Always in script mode.

**Exact output (locked decision from STATE.md):**
```sh
# cfgenable "MyConfig"  # Uncomment and run manually after verifying the config
cfgsave
```

### Pattern 5: Multi-VSAN IR → Single Brocade Fabric Output

**What:** MDS IR has zones keyed as `"name@vsanN"` with `Zone.VSAN > 0`. Brocade has no VSANs — all zones map to a single fabric. The emitter must iterate all zones regardless of their VSAN value and emit them as a flat list.

**When to use:** Whenever input was parsed from an MDS config (SourceFormat == "mds-nxos").

**Key insight:** Post-sanitization, MDS zones still use composite keys `"name@vsanN"` in the IR maps. The emitter uses `zone.Name` (the `.Name` field, not the map key) for the FOS command. Zone membership remains intact — VSAN isolation disappears in FOS.

### Pattern 6: Test via bytes.Buffer (established in parsers)

**What:** Tests capture emitter output into `bytes.Buffer` and assert on `buf.String()`.

**When to use:** All emitter tests — avoids file I/O in tests.

**Example:**
```go
var buf bytes.Buffer
err := Emit(cfg, &buf, false)
require.NoError(t, err)
output := buf.String()
require.Contains(t, output, `alicreate "host_01", "10:00:00:00:c9:ab:cd:ef"`)
```

### Anti-Patterns to Avoid

- **Ranging maps directly for output:** Go maps are unordered; always sort keys first. Failing to sort produces non-deterministic command ordering — ops team diffs become noise.
- **Using `html/template`:** Escapes `"`, `:`, `<`, `>` — will corrupt pWWN colon notation and quoted command arguments. Use `text/template` or `fmt.Fprintf`.
- **Emitting `cfgenable` as a live command:** This is a locked project decision. The ops team must enable the config manually after review. Auto-enabling would be dangerous in production fabric.
- **Omitting `defzone --noaccess`:** Without this preamble, Brocade switches default to all-access mode — any unzoned device can communicate with everything. This is the principal security control.
- **Omitting `cfgsave`:** Changes not followed by cfgsave are lost on switch reboot. The postamble is mandatory for persistence.
- **Empty member lists in zonecreate:** FOS rejects `zonecreate "z", ""`. Zones with zero non-unsupported members should be skipped with a warning (consistent with PARSE-05 behavior).

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Deterministic map ordering | Custom sort logic | `sort.Strings(keys)` | Standard idiom; already used in sanitizer |
| Output buffering | Manual byte array | `bytes.Buffer` (tests) / `bufio.Writer` (if perf needed) | stdlib; zero deps |
| String joining with separator | Manual loop with comma-check | `strings.Join(slice, ";")` | Correct edge cases (empty slice, single element) |
| Format string quoting | `"\"" + name + "\""` | `fmt.Fprintf(w, "%q", name)` or explicit `"` prefix/suffix | `%q` adds Go escape sequences which differ from FOS quoting; use explicit `"` prefix/suffix for FOS CLI strings |

**Key insight on quoting:** `%q` in Go uses Go string escaping (e.g., `\n`, `\t`) — **not correct for FOS CLI**. FOS expects literal double-quotes around names. Use `fmt.Fprintf(w, "alicreate \"%s\", \"%s\"\n", name, pwwn)` or the explicit string prefix/suffix approach, not `%q`. Verified from FOS 9.2.x `aliCreate` documentation.

---

## Brocade FOS CLI Command Reference

### Exact Syntax (HIGH confidence — verified from techdocs.broadcom.com FOS 9.2.x)

```sh
# Alias creation
alicreate "<aliasName>", "<pwwn>"

# Zone creation — members separated by semicolons, no trailing semicolon
zonecreate "<zoneName>", "<member1>;<member2>;<memberN>"

# Config creation — zone names separated by semicolons
cfgcreate "<cfgName>", "<zone1>;<zone2>;<zoneN>"

# Save to persistent storage (mandatory postamble)
cfgsave

# Enable configuration — NEVER emit as executable; always comment out
# cfgenable "<cfgName>"  # Uncomment and run manually after verifying the config

# Default zone security preamble (mandatory — must precede all zone commands)
defzone --noaccess
```

### Key FOS Syntax Rules (HIGH confidence)

1. **Names are quoted** with double-quotes in all three create commands.
2. **Comma separates** the name from the member list in all three create commands.
3. **Semicolons separate** members within the member list — no spaces required but spaces are accepted.
4. **pWWNs and alias names** are interchangeable in `zonecreate` member lists — FOS resolves alias names at cfgenable time.
5. **`defzone --noaccess` requires** a subsequent `cfgSave` or `cfgEnable` or `cfgDisable` to commit the change to the fabric. The `cfgsave` postamble satisfies this requirement.
6. **Empty member lists** are not allowed by FOS — `zonecreate "z", ""` will fail.

### Output Section Order (OUT-01)

```
defzone --noaccess        ← script mode only (preamble)
[blank line]
# --- Aliases ---
alicreate ...             ← all aliases
[blank line]
# --- Zones ---
zonecreate ...            ← all zones
[blank line]
# --- Configs ---
cfgcreate ...             ← all zone configs
[blank line]
# cfgenable "<cfg>"       ← script mode only (commented)
cfgsave                   ← script mode only (postamble)
```

---

## Common Pitfalls

### Pitfall 1: Non-Deterministic Output Order

**What goes wrong:** Ranging Go maps without sorting produces different command orderings on each run. Ops team sees spurious diffs when re-running the tool on the same input.

**Why it happens:** Go map iteration order is intentionally randomized since Go 1.

**How to avoid:** Always extract keys to `[]string`, call `sort.Strings()`, then range the slice.

**Warning signs:** Test flakiness — same IR occasionally produces different output orderings.

---

### Pitfall 2: MDS Composite Key vs Zone Name Mismatch

**What goes wrong:** MDS-sourced IR has map keys like `"fabric_zone1@vsan10"` but the FOS command needs just `"fabric_zone1"`. Using `key` instead of `zone.Name` emits garbage zone names.

**Why it happens:** The sanitizer preserves the MDS composite key format (`name@vsanN`) in the map key even after sanitization.

**How to avoid:** Always use `zone.Name` (the struct field) for the emitted command argument, never the map key.

**Warning signs:** Test failures showing `@vsan` in emitted output.

---

### Pitfall 3: %q Format Verb Corruption

**What goes wrong:** Using `fmt.Fprintf(w, "alicreate %q, %q\n", name, pwwn)` produces `alicreate "host_01", "10:00:00:00:c9:ab:cd:ef"` for ASCII-only strings but adds Go escape sequences for any non-ASCII or special character. This could corrupt names with backslashes or similar.

**Why it happens:** Go's `%q` produces a Go-syntax double-quoted string, not a bare double-quoted string. For plain ASCII names they look the same, but semantics differ.

**How to avoid:** Use explicit double-quote characters: `fmt.Fprintf(w, "alicreate \"%s\", \"%s\"\n", name, pwwn)`.

**Warning signs:** Test output shows `\x` or `\u` escape sequences in command arguments.

---

### Pitfall 4: Empty Zone Emission

**What goes wrong:** A zone whose every member has `Type == "unsupported"` would emit `zonecreate "z", ""` — rejected by FOS with an error.

**Why it happens:** Member skipping (warn-and-continue for unsupported types) can produce a zero-member zone.

**How to avoid:** After building the members slice, if it is empty, skip the zone entirely and append a warning: `"zone %q has no valid FOS members after filtering unsupported types — skipped"`.

**Warning signs:** `cfgcreate` references a zone name that was not emitted — FOS would reject the config.

---

### Pitfall 5: cfgcreate References Non-Existent Zone

**What goes wrong:** If a ZoneConfig's ZoneNames references a zone that was skipped (empty members pitfall above), the emitted `cfgcreate` will list a zone that doesn't exist in the FOS script — FOS will reject the config.

**Why it happens:** ZoneConfig.ZoneNames is updated by the sanitizer for renaming, but not pruned for zone-skip decisions made at emit time.

**How to avoid:** Track which zones were actually emitted. When emitting `cfgcreate`, filter ZoneNames to only include zones that were emitted. Warn for any dropped zone reference.

**Warning signs:** `cfgcreate` member list contains names not present in `zonecreate` commands above it.

---

## Code Examples

### Verified FOS CLI Output Format (from testdata/brocade/cli_basic.cfg and official docs)

```
alicreate "host_01", "10:00:00:00:c9:ab:cd:ef"
alicreate "storage_01", "50:05:07:61:01:23:45:67"
zonecreate "fabric_zone1", "host_01;storage_01"
cfgcreate "Production_cfg", "fabric_zone1"
```

This confirms:
- Double-quotes around names and member lists
- Comma+space between name and member list
- Semicolons between members (no spaces around semicolons in the existing fixture)

### Script Mode Preamble and Postamble

```sh
defzone --noaccess

alicreate "host_01", "10:00:00:00:c9:ab:cd:ef"
zonecreate "fabric_zone1", "host_01;storage_01"
cfgcreate "Production_cfg", "fabric_zone1"

# cfgenable "Production_cfg"  # Uncomment and run manually after verifying the config
cfgsave
```

### Emitter Function Signature

```go
// Source: internal/emitter/brocade/doc.go (pre-existing package declaration)
// Emit writes alicreate/zonecreate/cfgcreate commands for cfg to w.
// When scriptMode is true, the output is wrapped with defzone --noaccess preamble
// and cfgsave postamble. cfgenable is always emitted as a comment, never executable.
// Returns non-nil error only on write failures.
func Emit(cfg *ir.ZoningConfig, w io.Writer, scriptMode bool) error
```

### Test Pattern (capturing output to bytes.Buffer)

```go
// Source: established pattern from parser tests in this project
func TestEmit(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name       string
        input      *ir.ZoningConfig
        scriptMode bool
        checkFn    func(t *testing.T, output string)
    }{
        {
            name: "commands-only mode emits alicreate before zonecreate before cfgcreate",
            // ...
            checkFn: func(t *testing.T, output string) {
                t.Helper()
                aliPos := strings.Index(output, "alicreate")
                zonePos := strings.Index(output, "zonecreate")
                cfgPos := strings.Index(output, "cfgcreate")
                require.Less(t, aliPos, zonePos, "alicreate must appear before zonecreate")
                require.Less(t, zonePos, cfgPos, "zonecreate must appear before cfgcreate")
            },
        },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            var buf bytes.Buffer
            err := Emit(tt.input, &buf, tt.scriptMode)
            require.NoError(t, err)
            tt.checkFn(t, buf.String())
        })
    }
}
```

---

## Plan Structure Recommendation

Following the established 2-plan TDD pattern from Phases 2, 3, and 4:

**Plan 01 (TDD red phase):** Create `internal/emitter/brocade/emitter_test.go` with table-driven tests covering all 5 requirements (CONV-01, CONV-02, CONV-03, OUT-01, OUT-02). Tests must fail with `undefined: Emit`. IR structs are built inline (no fixture files needed for emitter tests).

**Plan 02 (TDD green phase):** Implement `internal/emitter/brocade/emitter.go` making all tests pass.

**Test cases required for Plan 01:**
1. Commands-only: all three command types emitted (CONV-01, CONV-02, CONV-03)
2. Commands-only: correct order alicreate → zonecreate → cfgcreate (OUT-01)
3. Script mode: `defzone --noaccess` present at start (OUT-02)
4. Script mode: `cfgsave` present at end (OUT-02)
5. Script mode: `cfgenable` present as comment, never executable (OUT-02)
6. Alias member in zone: `zonecreate` emits alias name, not raw pWWN (CONV-02)
7. pWWN member in zone: `zonecreate` emits pWWN value directly
8. Empty zone (all members unsupported): zone skipped with warning, no empty `zonecreate`
9. Multi-VSAN MDS IR: all zones emitted regardless of VSAN (PARSE-06 / CONV-02 integration)
10. Deterministic output: same IR produces same ordering on repeated calls

---

## Environment Availability

Step 2.6: SKIPPED (no external dependencies identified — pure code generation from in-memory IR to io.Writer)

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Write directly to os.Stdout | Write to io.Writer interface | CLAUDE.md (project founding) | Emitter is testable without stdout capture hacks |
| cfgenable in generated scripts | cfgenable commented out | STATE.md locked decision | Prevents accidental production fabric disruption |
| Emit without defzone preamble | defzone --noaccess mandatory | STATE.md locked decision | Fabric-wide security: no unzoned device communication |

---

## Open Questions

1. **cfgenable comment format**
   - What we know: STATE.md says "cfgenable appears in the generated script as a commented-out line with an explanatory comment"
   - What's unclear: Exact comment text and whether to include the cfg name in the comment
   - Recommendation: Include the cfg name for usability — ops team knows which config to enable. Format: `# cfgenable "<cfgName>"  # Uncomment and run manually after verifying the config`

2. **Multiple ZoneConfigs in one IR**
   - What we know: IR supports multiple ZoneConfigs; Brocade also supports multiple cfgs but only one can be active
   - What's unclear: If multiple cfgs exist, which one goes into cfgenable comment?
   - Recommendation: Emit a commented cfgenable for each ZoneConfig, sorted alphabetically. Ops team uncomments the one they want.

3. **Separator spacing in member lists**
   - What we know: Official FOS docs show `"member1; member2"` (space after semicolon) in examples but the existing testdata fixture uses `"host_01;storage_01"` (no space)
   - What's unclear: Whether FOS requires/prefers one form
   - Recommendation: No space (consistent with existing cli_basic.cfg fixture which was tested against the Brocade parser). FOS accepts both.

4. **Section comment headers**
   - What we know: The output structure has three logical sections (aliases, zones, configs)
   - What's unclear: Whether to emit `# --- Aliases ---` style comment headers
   - Recommendation: Include section comment headers for human readability; they are valid shell comments and help ops teams navigate the output file.

---

## Sources

### Primary (HIGH confidence)
- `techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-commands/9-2-x/Fabric-OS-Commands/aliCreate.html` — exact alicreate syntax, quoting rules, member separator
- `techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-commands/9-2-x/Fabric-OS-Commands/zoneCreate.html` — exact zonecreate syntax, member types, separator rules
- `techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-commands/9-2-x/Fabric-OS-Commands/defZone.html` — defzone --noaccess syntax, required follow-up commands
- `internal/ir/zoningconfig.go` — IR struct contract (verified source of truth for this project)
- `internal/emitter/brocade/doc.go` — pre-existing package declaration with behavior contract
- `internal/validator/sanitizer.go` — confirms Sanitize() output is safe for direct emission (no further name validation needed)
- `testdata/brocade/cli_basic.cfg` — confirms exact FOS CLI output format used as ground truth

### Secondary (MEDIUM confidence)
- `www.penguinpunk.net/blog/brocade-alias-and-zone-syntax-or-how-fos-is-a-love-hate-thing/` — confirmed hyphen-in-name pitfall (already handled by Phase 4 sanitizer), exact CLI quoting format
- `vmarena.com/blogs/how-to-create-zones-from-cli-on-a-brocade-san-switch/` — workflow verification: alicreate → zonecreate → cfgcreate → cfgenable → cfgsave order
- `manualowl.com/m/Hewlett-Packard/StorageWorks-4/8/Manual/85171?page=123` — defzone --noaccess with cfgsave follow-up requirement confirmed

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all stdlib, already in project, no new deps
- FOS CLI syntax: HIGH — verified against official Broadcom FOS 9.2.x techdocs
- Architecture: HIGH — follows established project patterns from Phases 2/3/4
- Pitfalls: HIGH — composite key pitfall verified from codebase; empty zone and member pitfalls derived from FOS documented constraints
- Plan structure: HIGH — mirrors exact 2-plan TDD pattern used in all prior phases

**Research date:** 2026-03-29
**Valid until:** 2026-12-01 (FOS command syntax is stable across versions; Go stdlib is stable)
