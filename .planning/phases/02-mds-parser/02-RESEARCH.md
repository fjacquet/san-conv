# Phase 2: MDS Parser - Research

**Researched:** 2026-03-28
**Domain:** Go line-by-line state-machine parser for Cisco MDS NX-OS running-config
**Confidence:** HIGH

---

## Summary

Phase 2 implements `internal/parser/mds` — the first functional component of the compiler pipeline. Its sole job is to read a Cisco MDS NX-OS running-config file and populate a `*ir.ZoningConfig`. The IR contract is already locked in `internal/ir/zoningconfig.go` from Phase 1; the parser must produce exactly that struct, nothing more.

The parsing problem is well-understood: NX-OS zoning config is a hierarchical but line-oriented text format. The canonical Go approach — `bufio.Scanner` with a state machine and package-level compiled regexes — handles it cleanly without external grammar libraries. Two distinct alias databases (fabric-wide `device-alias database` and per-VSAN `fcalias`) coexist in real enterprise configs and both must be collected in pass 1 before zone members can be resolved in pass 2.

The highest-risk correctness issue for this phase is the device-alias enhanced mode (NX-OS 8.5.1+ default): zone members appear as `member device-alias NAME` rather than `member pwwn`. A parser that only handles pWWN members silently produces empty zones — the most dangerous failure mode because no error is raised. The two-pass design is the direct prevention.

**Primary recommendation:** Implement a two-pass parser with an explicit state machine. Pass 1 scans the entire file and builds: (a) the device-alias map keyed by alias name, and (b) per-VSAN fcalias maps. Pass 2 rescans and populates zones, resolving `member device-alias` and `member fcalias` references by lookup. Build five fixture files covering the six distinct syntax cases before writing parser logic.

---

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PARSE-01 | Parse `device-alias database` section to extract fabric-wide alias → pWWN mappings | Pass 1 state machine: `stateDeviceAliasDB`; regex `^\s+device-alias name (\S+)\s+pwwn (\S+)` |
| PARSE-02 | Parse `fcalias name X vsan Y` definitions to extract per-VSAN alias → pWWN mappings | Pass 1 state machine: `stateFcAlias`; regex `^fcalias name (\S+) vsan (\d+)` + `^\s+member pwwn (\S+)` |
| PARSE-03 | Parse `zone name X vsan Y` blocks including all three member types | Pass 2 state machine: `stateZone`; three member sub-regexes (device-alias, fcalias, pwwn) |
| PARSE-04 | Parse `zoneset name X vsan Y` blocks and their zone membership | Pass 2 state machine: `stateZoneset`; regex `^\s+member (\S+)` |
| PARSE-05 | Detect unsupported member types (interface, fcid, ip-address, symbolic-nodename) and emit warning, skip member | Named keywords checked inside `stateZone` member handling; append to `cfg.Warnings` |
| PARSE-06 | Handle multi-VSAN configs; all VSANs merged into single IR | VSAN extracted from zone/zoneset name headers; stored in `Zone.VSAN` and `ZoneConfig.VSAN` fields; no per-VSAN isolation in IR |
</phase_requirements>

---

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `bufio` | Go 1.26.1 (project module) | Line-by-line scanning of config text | Canonical Go approach; handles CRLF transparently; no deps |
| stdlib `regexp` | Go 1.26.1 | Pattern matching against config lines | Compiled once at package init; fast enough for any real config |
| stdlib `strings` | Go 1.26.1 | Trimming, splitting member lists | Zero dep string operations |
| stdlib `fmt` | Go 1.26.1 | Warning message formatting | Standard |
| `internal/ir` | local | Target struct definitions | The locked IR contract from Phase 1 |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/stretchr/testify` | v1.11.1 | Test assertions | All parser tests — `require.Equal`, `require.NoError`, `require.Len` |
| stdlib `os` | Go 1.26.1 | Open fixture files in tests | `os.Open(filepath)` for table-driven tests |
| stdlib `path/filepath` | Go 1.26.1 | Construct fixture paths | `filepath.Join("testdata", "mds", "basic.cfg")` |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `bufio.Scanner` | `io.Reader` + manual `bytes.Split` | Scanner is cleaner for line-at-a-time; handles CRLF automatically |
| package-level `regexp.MustCompile` | Inline `regexp.Compile` | Package-level avoids recompilation per call; MustCompile panics at init if regex is malformed (caught immediately in tests) |
| two-pass over `io.Reader` | single-pass with deferred resolution | Two-pass requires re-reading or buffering; the input is a file, so re-open or buffer in memory — buffer the lines into `[]string` once, iterate twice |

**Installation:** No new dependencies — all required libraries are stdlib or already present from Phase 1.

---

## Architecture Patterns

### Recommended Project Structure
```
internal/parser/mds/
├── parser.go         # Parse() func, state machine, all regexes
└── parser_test.go    # Table-driven tests, one case per fixture

testdata/mds/
├── basic.cfg             # device-alias + fcalias + zones + zoneset (basic mode pWWN members)
├── enhanced_mode.cfg     # device-alias + zones with member device-alias (NX-OS 8.5.1+)
├── multi_vsan.cfg        # Two VSANs with distinct zones; one shared device-alias DB
├── smart_zoning.cfg      # member pwwn X init / target / both keywords
├── unsupported.cfg       # member interface / fcid / ip-address lines
└── edge_cases.cfg        # Empty zone, zone not in any zoneset, IVR zone, device-alias commit line
```

### Pattern 1: Two-Pass Line Scanner

**What:** Buffer all lines into `[]string` on first read, then iterate twice. Pass 1 populates alias databases. Pass 2 populates zones and zonesets with resolved members.

**When to use:** Any time zone members reference names that are defined later (or earlier) in the file. NX-OS always places `device-alias database` before zone definitions, but fcalias blocks may interleave with zones in complex configs.

**Implementation sketch:**

```go
// Source: bufio.Scanner pattern — Go stdlib docs
func Parse(r io.Reader) (*ir.ZoningConfig, error) {
    // Read all lines once into memory
    var lines []string
    scanner := bufio.NewScanner(r)
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("reading config: %w", err)
    }

    cfg := &ir.ZoningConfig{
        SourceFormat: "mds-nxos",
        Aliases:      make(map[string]*ir.Alias),
        Zones:        make(map[string]*ir.Zone),
        ZoneConfigs:  make(map[string]*ir.ZoneConfig),
    }

    pass1BuildAliases(lines, cfg)  // populates cfg.Aliases
    pass2BuildZones(lines, cfg)    // populates cfg.Zones, cfg.ZoneConfigs
    return cfg, nil
}
```

### Pattern 2: Explicit State Machine

**What:** An integer or string `state` variable tracks which config block is currently open. Every line is classified by matching against the top-level header regexes first; if no header matches, the current-state member regexes are tried.

**States required for pass 1:**
- `stateIdle` — scanning for top-level keywords
- `stateDeviceAliasDB` — inside `device-alias database` ... `device-alias commit`
- `stateFcAlias` — inside `fcalias name X vsan N` block

**States required for pass 2:**
- `stateIdle`
- `stateZone` — inside `zone name X vsan N` block
- `stateZoneset` — inside `zoneset name X vsan N` block

**Block-exit condition:** Any line that is not blank AND does not start with whitespace AND does not match the current block's member pattern returns the machine to `stateIdle`. This is the standard NX-OS block-end heuristic — a new top-level keyword always starts at column 0.

```go
// Determining block end
func isTopLevelKeyword(line string) bool {
    trimmed := strings.TrimSpace(line)
    if trimmed == "" || strings.HasPrefix(trimmed, "!") {
        return false // blank line or comment — stay in block
    }
    // Top-level lines have no leading whitespace
    return len(line) > 0 && line[0] != ' ' && line[0] != '\t'
}
```

**Important:** `device-alias commit` is a top-level keyword that terminates the device-alias database block. It must be matched (and silently skipped) rather than mis-parsed as a device-alias entry.

### Pattern 3: Compiled Package-Level Regexes

**What:** All `regexp.MustCompile` calls at package `var` init time, not inside loops.

**Why:** Prevents recompilation per line on large configs (10,000+ zone lines). Also surfaces malformed regex at program startup rather than mid-parse.

```go
// Source: standard Go idiom for parser regexes
var (
    // Pass 1 — alias databases
    reDeviceAliasDBHeader = regexp.MustCompile(`^device-alias\s+database\s*$`)
    reDeviceAliasEntry    = regexp.MustCompile(`^\s+device-alias\s+name\s+(\S+)\s+pwwn\s+(\S+)`)
    reDeviceAliasCommit   = regexp.MustCompile(`^device-alias\s+commit\s*$`)
    reFcAliasHeader       = regexp.MustCompile(`^fcalias\s+name\s+(\S+)\s+vsan\s+(\d+)`)
    reFcAliasMember       = regexp.MustCompile(`^\s+member\s+pwwn\s+(\S+)`)

    // Pass 2 — zones and zonesets
    reZoneHeader          = regexp.MustCompile(`^zone\s+name\s+(\S+)\s+vsan\s+(\d+)`)
    reZonesetHeader       = regexp.MustCompile(`^zoneset\s+name\s+(\S+)\s+vsan\s+(\d+)`)
    reZonesetActivate     = regexp.MustCompile(`^zoneset\s+activate\s+name\s+(\S+)\s+vsan\s+(\d+)`)

    // Zone member types
    reMemberDeviceAlias   = regexp.MustCompile(`^\s+member\s+device-alias\s+(\S+)`)
    reMemberFcAlias       = regexp.MustCompile(`^\s+member\s+fcalias\s+(\S+)`)
    reMemberPWWN          = regexp.MustCompile(`^\s+member\s+pwwn\s+(\S+)(?:\s+(?:init|target|both))?`)
    reMemberPWWNRole      = regexp.MustCompile(`^\s+member\s+pwwn\s+\S+\s+(init|target|both)\s*$`)

    // Unsupported member types — detect before falling through
    reMemberInterface     = regexp.MustCompile(`^\s+member\s+interface\s+`)
    reMemberFcid          = regexp.MustCompile(`^\s+member\s+fcid\s+`)
    reMemberIPAddr        = regexp.MustCompile(`^\s+member\s+ip-address\s+`)
    reMemberSymbolicNode  = regexp.MustCompile(`^\s+member\s+symbolic-nodename\s+`)
    reMemberFwwn          = regexp.MustCompile(`^\s+member\s+fwwn\s+`)

    // IVR zones — must be explicitly excluded
    reIVRZoneHeader       = regexp.MustCompile(`^ivr\s+zone\s+name\s+`)
    reIVRZonesetHeader    = regexp.MustCompile(`^ivr\s+zoneset\s+name\s+`)

    // Zoneset activate — for ZoneConfig.Active
    reZonesetMember       = regexp.MustCompile(`^\s+member\s+(\S+)`)
)
```

### Pattern 4: Member Resolution with Warning on Undefined Reference

**What:** During pass 2, when a `member device-alias NAME` is encountered, look up `NAME` in `cfg.Aliases`. If not found, append a warning to `cfg.Warnings` and add an `"unsupported"` member with the raw string, so the zone still appears in IR (not silently dropped).

```go
// Zone member resolution in pass 2
func resolveMember(aliasName string, cfg *ir.ZoningConfig, zoneName string) *ir.ZoneMember {
    if alias, ok := cfg.Aliases[aliasName]; ok {
        _ = alias // alias exists — store reference by name in member
        return &ir.ZoneMember{Type: "alias", Value: aliasName}
    }
    // Undefined device-alias reference — warn, keep member
    cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
        "zone %q: member device-alias %q not found in device-alias database — member kept as unresolved alias reference",
        zoneName, aliasName,
    ))
    return &ir.ZoneMember{Type: "alias", Value: aliasName}
}
```

**Note on IR design:** `ZoneMember.Type = "alias"` with `Value = aliasName` is the correct IR representation for both resolved and unresolved device-alias/fcalias references. The emitter uses the alias name; the validator (Phase 4) checks if it exists in `cfg.Aliases`.

### Pattern 5: Smart Zoning Keyword Stripping

**What:** `member pwwn 50:00:... init` — the role keyword must be stripped and a warning emitted. The pWWN itself is preserved.

```go
// Inside stateZone member handling
if m := reMemberPWWN.FindStringSubmatch(line); m != nil {
    pwwn := normalizeWWN(m[1])
    // Check for smart-zoning role keyword
    if roleMatch := reMemberPWWNRole.FindStringSubmatch(line); roleMatch != nil {
        cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
            "zone %q: smart-zoning role %q on member %s stripped — no FOS equivalent",
            currentZone.Name, roleMatch[1], pwwn,
        ))
    }
    currentZone.Members = append(currentZone.Members, &ir.ZoneMember{Type: "pwwn", Value: pwwn})
    continue
}
```

### Pattern 6: Warning Struct — Named Warnings with Context

**What:** The IR `Warnings []string` field (from `zoningconfig.go`) stores all non-fatal parse issues. Each warning string must include: object type, object name, issue description, and (for skip decisions) what action was taken.

**Format convention:**
```
"<context>: <message> — <action taken>"
```

**Named warning categories (for this phase):**
| Warning Name | Trigger | Format |
|---|---|---|
| `smart-zoning-role` | `member pwwn X init/target/both` | `zone %q: smart-zoning role %q on member %s stripped — no FOS equivalent` |
| `unsupported-member` | `member interface/fcid/ip-address/symbolic-nodename/fwwn` | `zone %q: unsupported member type %q (%s) skipped` |
| `ivr-zone-skipped` | `ivr zone name X` | `IVR zone %q skipped — no FOS equivalent` |
| `unresolved-alias` | device-alias name in zone not in alias DB | `zone %q: member device-alias %q not found in device-alias database` |
| `unresolved-fcalias` | fcalias name in zone not in fcalias DB | `zone %q: member fcalias %q (vsan %d) not found` |
| `multi-vsan` | second unique VSAN seen | `multi-VSAN config detected (%d VSANs) — zones are VSAN-scoped; all converted to single Brocade fabric` |

### Anti-Patterns to Avoid

- **Single-pass pWWN-only parsing:** Silently drops all enhanced-mode device-alias zone members. Always implement the two-pass design.
- **Global state for parser context:** Use a local `state` variable and local pointer variables (`currentZone`, `currentFcAlias`) within the `pass1BuildAliases` / `pass2BuildZones` functions. Never use package-level variables — they make the parser non-reentrant.
- **Returning error on unknown line:** Unknown lines (interface config, feature config, NTP, etc.) must be silently skipped. Only emit errors for genuine I/O failures.
- **Treating `device-alias commit` as a device-alias entry:** It matches a leading `device-alias` pattern. The commit line must be explicitly matched and skipped before the entry pattern is tried.
- **Merging fcalias lookups across VSANs:** fcalias is VSAN-scoped. An fcalias named `Storage-Port` in VSAN 10 is a different object from an fcalias named `Storage-Port` in VSAN 20. Index them as `map[int]map[string]*ir.Alias` keyed by VSAN.
- **Regex matching `zone name` before `ivr zone name`:** IVR zone lines contain the substring `zone name`. Always match the `ivr zone name` pattern first in the line classification logic.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Line scanning | Custom byte reader | `bufio.Scanner` | Handles CRLF, EOF, partial reads; battle-tested |
| Regex compilation | Per-call `regexp.Compile` | `regexp.MustCompile` at package init | One-time cost; panics at startup if broken |
| Test assertions | Manual `if actual != expected { t.Fatalf(...)` | `testify/require` | Stops test on first failure; cleaner diff output |
| WWN normalization | Custom character replacer | `strings.ToLower` + validate against `^([0-9a-f]{2}:){7}[0-9a-f]{2}$` | pWWNs can arrive as uppercase; normalize on parse |

**Key insight:** The parser has exactly one job: classify lines, extract named capture groups, and populate IR struct fields. Every line of logic beyond that is scope creep.

---

## Common Pitfalls

### Pitfall 1: Enhanced Mode Zone Members — Silent Empty Zones

**What goes wrong:** Parser only handles `member pwwn`; NX-OS 8.5.1+ configs use `member device-alias`. Result: zones appear in IR with empty member lists. No error raised.

**Why it happens:** Test fixtures are written in basic mode (older syntax); enhanced mode is production default.

**How to avoid:** Two-pass design. Fixture `enhanced_mode.cfg` must be tested explicitly. Check that zone member count matches source.

**Warning signs:** Zero members in converted zones despite non-empty source zones.

### Pitfall 2: `device-alias commit` Mis-Parsed as Entry

**What goes wrong:** The commit line `device-alias commit` begins with `device-alias` and lands inside the `stateDeviceAliasDB` block. A regex like `^device-alias\s+name\s+(\S+)` won't match it, but a looser regex could. More importantly, the commit line is also the block terminator signal — it must exit the state.

**How to avoid:** Match `reDeviceAliasCommit` explicitly and transition to `stateIdle`.

### Pitfall 3: IVR Zone Headers Match Zone Regex

**What goes wrong:** `ivr zone name FOO vsan 10` contains the substring `zone name FOO vsan 10`. If `reZoneHeader` is applied before `reIVRZoneHeader`, IVR zones are parsed as regular zones.

**How to avoid:** In pass 2, classify lines in this order: (1) check IVR patterns → skip+warn; (2) check zone/zoneset headers; (3) fall through to member patterns.

### Pitfall 4: Smart Zoning Role Keyword Corrupts pWWN

**What goes wrong:** `member pwwn 50:00:c9:00:00:00:00:01 init` — naive split on whitespace produces `["member", "pwwn", "50:00:c9:00:00:00:00:01", "init"]`. If the parser takes token index 2 as the whole member string, it gets the correct pWWN. But if it trims whitespace from the right of the line and parses greedily, it might pick up `init` as part of the WWN.

**How to avoid:** Named capture group regex: `^\s+member\s+pwwn\s+(\S+)` captures only the WWN token. The optional role keyword is a separate capture group.

### Pitfall 5: Multi-VSAN Cross-Contamination

**What goes wrong:** Two zones named `Storage-Zone` exist in VSAN 10 and VSAN 20 respectively. If the IR uses a plain `map[string]*ir.Zone` keyed by name, the second zone silently overwrites the first.

**How to avoid:** The IR `Zone` struct carries a `VSAN int` field. Zones must be keyed with a composite key or the zone name must be stored with VSAN context. For the map key, use `fmt.Sprintf("%s@vsan%d", name, vsan)` as the internal map key, but store the original `Name` and `VSAN` in the struct fields. This way the emitter sees both VSANs' zones.

**Note:** This is a design decision point. The IR's `Zones map[string]*Zone` field uses zone name as key. For multi-VSAN, two zones can share a name. Either: (a) use a composite key in the map, or (b) rename one with a VSAN suffix and warn. Option (a) is cleaner for Phase 2; Phase 7 CLI wiring will decide the output strategy.

### Pitfall 6: fcalias vs. device-alias Coexistence

**What goes wrong:** A zone has `member fcalias Storage-Port` (VSAN-scoped) and `member device-alias Host-HBA` (fabric-wide). A parser that only builds one alias map drops members of the other type.

**How to avoid:** Pass 1 populates two separate data structures:
- `deviceAliasMap map[string]*ir.Alias` — from `device-alias database` block
- `fcAliasMap map[int]map[string]*ir.Alias` — from all `fcalias` blocks, keyed by VSAN

Pass 2 member resolution checks both: fcalias reference → look in `fcAliasMap[vsan]`; device-alias reference → look in `deviceAliasMap`.

### Pitfall 7: Blank Lines Inside Blocks

**What goes wrong:** Some NX-OS configs include blank lines between member entries within a zone block. A state machine that exits on "no leading whitespace" might exit early if it also exits on blank lines.

**How to avoid:** The block-exit condition should be: `len(trimmed) > 0 && line[0] != ' ' && line[0] != '\t'`. A blank line inside a block is a no-op — stay in the current state.

---

## Code Examples

Verified patterns based on the confirmed NX-OS syntax from the prompt and the locked IR from `internal/ir/zoningconfig.go`:

### Fixture: basic.cfg (PARSE-01, PARSE-02, PARSE-03, PARSE-04)
```
device-alias database
  device-alias name Server-HBA-A pwwn 50:05:0c:00:00:c8:aa:50
  device-alias name Storage-Port-1 pwwn 50:06:0e:80:04:7c:00:01
device-alias commit

fcalias name Server-port-A vsan 10
  member pwwn 50:05:0c:00:00:c8:aa:50

zone name Server vsan 10
  member device-alias Server-HBA-A
  member fcalias Server-port-A
  member pwwn 50:05:0c:00:00:c8:aa:51

zoneset name SAN-VSAN10 vsan 10
  member Server

zoneset activate name SAN-VSAN10 vsan 10
```

### Fixture: enhanced_mode.cfg (PARSE-01, PARSE-03 — NX-OS 8.5.1+ default)
```
device-alias database
  device-alias name Server-HBA-A pwwn 50:05:0c:00:00:c8:aa:50
  device-alias name Storage-Port-1 pwwn 50:06:0e:80:04:7c:00:01
device-alias commit

zone name Storage-Zone vsan 10
  member device-alias Server-HBA-A
  member device-alias Storage-Port-1

zoneset name SAN-VSAN10 vsan 10
  member Storage-Zone

zoneset activate name SAN-VSAN10 vsan 10
```

### Fixture: multi_vsan.cfg (PARSE-06)
```
device-alias database
  device-alias name Host-A pwwn 20:00:00:00:c9:12:34:56
  device-alias name Storage-A pwwn 50:06:01:65:3e:a0:1e:d7
device-alias commit

zone name Zone-A vsan 10
  member device-alias Host-A
  member device-alias Storage-A

zoneset name ZS-VSAN10 vsan 10
  member Zone-A

zoneset activate name ZS-VSAN10 vsan 10

zone name Zone-B vsan 20
  member device-alias Host-A
  member device-alias Storage-A

zoneset name ZS-VSAN20 vsan 20
  member Zone-B

zoneset activate name ZS-VSAN20 vsan 20
```

### Fixture: smart_zoning.cfg (PARSE-03, PARSE-05 — smart zoning keywords)
```
zone name SmartZone vsan 10
  member pwwn 50:05:0c:00:00:c8:aa:50 init
  member pwwn 50:06:0e:80:04:7c:00:01 target
  member pwwn 50:06:0e:80:04:7c:00:02 both

zoneset name SAN-VSAN10 vsan 10
  member SmartZone
```

### Fixture: unsupported.cfg (PARSE-05)
```
zone name Mixed vsan 10
  member device-alias Server-HBA-A
  member interface fc1/1
  member fcid 0x6f0100 vsan 10
  member ip-address 192.168.1.1 255.255.255.0
  member symbolic-nodename myhost

zoneset name SAN-VSAN10 vsan 10
  member Mixed
```

### Fixture: edge_cases.cfg (IVR, empty zone, zone not in zoneset)
```
! This is a comment — should be silently skipped
device-alias database
  device-alias name Host-A pwwn 20:00:00:00:c9:12:34:56
device-alias commit

ivr zone name IVR-CrossFabric vsan 10
  member pwwn 20:00:00:00:c9:12:34:56

zone name EmptyZone vsan 10

zone name ActiveZone vsan 10
  member device-alias Host-A

zone name OrphanZone vsan 10
  member device-alias Host-A

zoneset name SAN-VSAN10 vsan 10
  member ActiveZone

zoneset activate name SAN-VSAN10 vsan 10
```

### Parser package signature
```go
// Source: project architecture contract in ARCHITECTURE.md
// internal/parser/mds/parser.go

package mds

import (
    "io"
    "github.com/fjacquet/san-conv/internal/ir"
)

// Parse reads a Cisco MDS NX-OS running-config from r and returns a populated
// *ir.ZoningConfig. Non-fatal issues are appended to cfg.Warnings.
// Parse only returns an error for I/O failures.
func Parse(r io.Reader) (*ir.ZoningConfig, error)
```

### WWN normalization helper
```go
// normalizeWWN normalizes a port WWN to lowercase colon-separated format.
// Input: "50:05:0C:00:00:C8:AA:50" or "50050c0000c8aa50"
// Output: "50:05:0c:00:00:c8:aa:50"
func normalizeWWN(raw string) string {
    // Remove any existing colons, lowercase, then reinsert
    compact := strings.ReplaceAll(strings.ToLower(raw), ":", "")
    if len(compact) != 16 {
        return strings.ToLower(raw) // malformed — return as-is, validator will catch it
    }
    parts := make([]string, 8)
    for i := range 8 {
        parts[i] = compact[i*2 : i*2+2]
    }
    return strings.Join(parts, ":")
}
```

### Table-driven test structure
```go
// Source: standard Go table-driven test pattern
// internal/parser/mds/parser_test.go

func TestParse(t *testing.T) {
    tests := []struct {
        name          string
        fixture       string
        wantAliases   int
        wantZones     int
        wantConfigs   int
        wantWarnings  int
        checkFn       func(t *testing.T, cfg *ir.ZoningConfig)
    }{
        {
            name:    "basic mode with device-alias and fcalias",
            fixture: "basic.cfg",
            wantAliases: 2, wantZones: 1, wantConfigs: 1, wantWarnings: 0,
        },
        {
            name:    "enhanced mode device-alias zone members",
            fixture: "enhanced_mode.cfg",
            wantAliases: 2, wantZones: 1, wantConfigs: 1, wantWarnings: 0,
            checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
                z := cfg.Zones["Storage-Zone"]
                require.NotNil(t, z)
                require.Len(t, z.Members, 2)
                require.Equal(t, "alias", z.Members[0].Type)
            },
        },
        {
            name:    "smart zoning keywords stripped with warning",
            fixture: "smart_zoning.cfg",
            wantZones: 1, wantWarnings: 3, // one per smart-zoning member
            checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
                z := cfg.Zones["SmartZone"]
                require.Len(t, z.Members, 3)
                require.Equal(t, "pwwn", z.Members[0].Type)
                // Warning contains "smart-zoning role"
                require.Contains(t, cfg.Warnings[0], "smart-zoning role")
            },
        },
        {
            name:    "unsupported members skipped with warnings",
            fixture: "unsupported.cfg",
            wantWarnings: 4, // interface, fcid, ip-address, symbolic-nodename
            checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
                z := cfg.Zones["Mixed"]
                require.NotNil(t, z, "zone must appear in IR even with only unsupported members skipped")
                require.Len(t, z.Members, 1, "only device-alias member should survive")
            },
        },
        {
            name:    "multi-VSAN produces distinct zones",
            fixture: "multi_vsan.cfg",
            wantZones: 2, wantConfigs: 2,
        },
        {
            name:    "IVR zone skipped with warning",
            fixture: "edge_cases.cfg",
            checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
                for name := range cfg.Zones {
                    require.NotContains(t, name, "IVR")
                }
                hasIVRWarning := false
                for _, w := range cfg.Warnings {
                    if strings.Contains(w, "IVR") {
                        hasIVRWarning = true
                    }
                }
                require.True(t, hasIVRWarning)
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            f, err := os.Open(filepath.Join("..", "..", "..", "testdata", "mds", tt.fixture))
            require.NoError(t, err)
            defer f.Close()

            cfg, err := Parse(f)
            require.NoError(t, err)
            if tt.wantAliases > 0 { require.Len(t, cfg.Aliases, tt.wantAliases) }
            if tt.wantZones > 0 { require.Len(t, cfg.Zones, tt.wantZones) }
            if tt.wantConfigs > 0 { require.Len(t, cfg.ZoneConfigs, tt.wantConfigs) }
            if tt.wantWarnings > 0 { require.Len(t, cfg.Warnings, tt.wantWarnings) }
            if tt.checkFn != nil { tt.checkFn(t, cfg) }
        })
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Basic mode: zone members as `member pwwn` | Enhanced mode: zone members as `member device-alias NAME` | NX-OS 8.5(1), 2019 | Two-pass parser required; single-pass pWWN parsers silently wrong |
| device-alias names up to 64 chars | 63-char limit introduced | NX-OS 9.2(2), ~2023 | Phase 4 sanitizer must enforce 63-char limit |
| Smart zoning keywords inline on member lines | No syntax change — still present in 9.x | Introduced NX-OS 5.2(6) | Must be stripped; no FOS equivalent |

**Deprecated/outdated:**
- `member pwwn` as the only zone member type: Works for basic-mode configs only. Enhanced mode (production default since 8.5.1) requires `member device-alias` handling.

---

## Open Questions

1. **Multi-VSAN map key strategy**
   - What we know: IR `Zones map[string]*Zone` uses zone name as key; two VSANs can have zones with identical names.
   - What's unclear: Should Phase 2 use a composite key `"name@vsanN"` internally, or should the map key always be the plain zone name (with later VSAN overwriting earlier on collision)?
   - Recommendation: Use composite key `zoneName+"@vsan"+strconv.Itoa(vsan)` in the map for Phase 2. This preserves all zones. The emitter (Phase 5) iterates `cfg.Zones` and will see all of them. Phase 7 CLI wiring decides output strategy for multi-VSAN.

2. **fcalias member pWWN resolution: store pWWN or alias name in ZoneMember?**
   - What we know: `fcalias name X vsan N` has `member pwwn Y` sub-entries. Zone references it as `member fcalias X`.
   - What's unclear: Should the zone member's `ZoneMember.Value` be the pWWN (fully resolved) or the fcalias name?
   - Recommendation: Store the fcalias name as `Value` with `Type = "alias"`, and add the fcalias to `cfg.Aliases` during pass 1 (same as device-alias). This makes all alias types uniform in the IR. The emitter uses alias name → alicreate lookup.

3. **`zoneset activate` with no matching zoneset definition**
   - What we know: `zoneset activate name X vsan N` sets `ZoneConfig.Name` as the active config.
   - What's unclear: Is it possible for `zoneset activate` to reference a zoneset not in the `zoneset name` blocks?
   - Recommendation: Store the active zoneset name in a local variable during pass 2; after all zones/zonesets are parsed, look up the ZoneConfig by name (accounting for composite key if multi-VSAN) and set a flag. If not found, emit a warning — do not error.

---

## Environment Availability

Step 2.6: SKIPPED — Phase 2 is a pure Go code implementation. The only external dependency is the Go toolchain (already verified as available from Phase 1, `go build ./...` passes).

---

## Validation Architecture

Nyquist validation is disabled (`workflow.nyquist_validation: false` in `.planning/config.json`). Section omitted per instructions.

---

## Project Constraints (from CLAUDE.md)

Directives from `./CLAUDE.md` that the planner must verify compliance with:

| Directive | Source | Impact on Phase 2 |
|-----------|--------|-------------------|
| **Go — single binary, no runtime deps** | CLAUDE.md Constraints | Parser must use only stdlib + existing `go.mod` dependencies (cobra, testify). No new runtime deps. |
| **Warn and continue — partial output better than stopping** | CLAUDE.md Constraints | `Parse()` must never return error for unconvertible constructs; all non-fatal issues go to `cfg.Warnings []string`. |
| **Input: full config file (offline/static)** | CLAUDE.md Constraints | Parser accepts `io.Reader`; no network calls; no switch API usage. |
| **Handle real-world MDS configs including edge cases** | CLAUDE.md Constraints | All six fixture files required; edge_cases.cfg must include comments, blank lines, IVR zones. |
| **Context7 before writing any library call** | CLAUDE.md (global) | Verify `bufio.Scanner`, `regexp.MustCompile` usage against stdlib docs before implementing. |
| **GSD workflow enforcement** | CLAUDE.md | Use `/gsd:execute-phase` entry point; do not make direct repo edits outside a GSD workflow. |
| **IR-first design is mandatory** | STATE.md Decisions | Phase 2 parser must populate exactly the structs in `internal/ir/zoningconfig.go`. No IR changes in Phase 2. |
| **No import cycles** | STATE.md Decisions | `internal/parser/mds` imports `internal/ir` only. No import of `cmd/`, `emitter/`, or `validator/`. |
| **Use RunE (not Run) on Cobra commands** | STATE.md Decisions | Not directly relevant to parser package, but the stub in `cmd/mds2brocade.go` already uses `RunE`. |
| **golangci-lint v2 config format** | STATE.md Decisions | New test file must pass `go tool golangci-lint run`. Add `//nolint` comments only when justified. |

---

## Sources

### Primary (HIGH confidence)
- `internal/ir/zoningconfig.go` — locked IR contract (local file, read directly)
- `.planning/research/ARCHITECTURE.md` — state machine patterns, regex examples, data flow (project research, 2026-03-28)
- `.planning/research/PITFALLS.md` — 15 verified pitfalls with official source citations (project research, 2026-03-28)
- `.planning/research/SUMMARY.md` — recommended stack, feature requirements (project research, 2026-03-28)
- `<known_syntax>` block in prompt — confirmed NX-OS running-config syntax for all 6 constructs
- Go stdlib bufio docs — `bufio.Scanner` line scanning (stdlib)
- Go stdlib regexp docs — `regexp.MustCompile` package-level pattern (stdlib)

### Secondary (MEDIUM confidence)
- Cisco MDS 9000 NX-OS Fabric Configuration Guide 9.x — device-alias enhanced mode, zone/zoneset syntax (cited in PITFALLS.md with URL)
- Cisco Smart Zoning technical note — `init`/`target`/`both` keyword syntax (cited in PITFALLS.md with URL)
- Cisco MDS NX-OS 9.2(2) Release Notes — 63-char name limit change (cited in PITFALLS.md with URL)

### Tertiary (LOW confidence)
- None — all findings backed by project research or stdlib docs.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libraries are stdlib or already in go.mod from Phase 1
- Architecture: HIGH — patterns directly from verified ARCHITECTURE.md research and locked IR
- Pitfalls: HIGH — 7 of 15 pitfalls from PITFALLS.md directly apply to Phase 2; all have official source citations
- Fixture design: HIGH — derived directly from confirmed NX-OS syntax in prompt and pitfall analysis
- Open questions: MEDIUM — three design decisions identified; all have recommended resolutions

**Research date:** 2026-03-28
**Valid until:** 2026-06-28 (stable NX-OS config syntax; Go stdlib stable)
