# Phase 3: Brocade Parser - Research

**Researched:** 2026-03-29
**Domain:** Brocade FOS cfgshow / CLI script parsing in Go
**Confidence:** HIGH

## Summary

Phase 3 implements the Brocade FOS parser inside the existing `internal/parser/brocade` package stub (currently only `doc.go`). The parser must handle two distinct text formats produced by Brocade FOS switches — the human-readable `cfgshow` output and machine-runnable FOS CLI scripts — and auto-detect which format it received. Both formats must produce an identical `*ir.ZoningConfig` struct with `SourceFormat: "brocade-fos"` and VSAN 0 as the sentinel for all zones (Brocade has no VSAN concept).

The MDS parser in `internal/parser/mds/parser.go` is the canonical reference implementation. The Brocade parser should follow its structural template: a single exported `Parse(r io.Reader)` function, package-level compiled regexps, `bufio.Scanner` line-by-line reading, and `require`-based table-driven tests against fixtures in `testdata/brocade/`. The two-pass approach of the MDS parser is NOT needed here — both Brocade formats define objects before they reference them (aliases exist before zones, zones before cfgs), so a single-pass state machine suffices.

The hardest parsing challenge is the cfgshow backslash continuation: a line ending with `\` means the next line is a continuation of the current member list. Member lists within a continuation block are semicolon-separated on one or more lines. The CLI format is simpler — each `alicreate`/`zonecreate`/`cfgcreate` command stands alone on one line with quoted name and quoted member list.

**Primary recommendation:** Single-pass state machine mirroring the MDS parser structure. Two plans: (1) create Brocade test fixtures first (TDD red-phase), (2) implement parser.go + parser_test.go against those fixtures.

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| PARSE-07 | Tool parses Brocade cfgshow output format (Defined configuration: section with alias:, zone:, cfg: lines including backslash-continuation) | cfgshow format is fully documented below; backslash continuation handling pattern is established |
| PARSE-08 | Tool parses Brocade FOS CLI script format (alicreate, zonecreate, cfgcreate commands) | CLI format uses fixed quoted-argument syntax; single regexp per command is sufficient |
| PARSE-09 | Tool auto-detects whether Brocade input is cfgshow output or CLI script format | Detection heuristic: scan for Defined configuration: header vs alicreate/zonecreate/cfgcreate verbs; mutually exclusive |
</phase_requirements>

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `bufio` | Go 1.26.1 (project go.mod) | Line-by-line scanner | Same as MDS parser; already in use |
| stdlib `regexp` | Go 1.26.1 | Pattern matching for cfgshow/CLI constructs | Same as MDS parser; already in use |
| stdlib `strings` | Go 1.26.1 | TrimSpace, Split, TrimSuffix for member lists | Same as MDS parser |
| stdlib `io` | Go 1.26.1 | `io.Reader` interface for testability | Mandatory pattern from CLAUDE.md and MDS parser |
| `github.com/stretchr/testify` | v1.11.1 (in go.mod) | `require.Equal`, `require.Len`, `require.NoError` in tests | Locked project convention — use `require` not `assert` |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| stdlib `fmt` | Go 1.26.1 | Warning message formatting | Same pattern as MDS parser for cfg.Warnings |
| stdlib `path/filepath` | Go 1.26.1 | Fixture paths in tests | Same pattern as MDS parser_test.go |
| stdlib `os` | Go 1.26.1 | `os.Open` in tests | Same pattern as MDS parser_test.go |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Single-pass state machine | Two-pass like MDS parser | Two-pass not needed: Brocade formats define aliases before zones, zones before cfgs; single pass is simpler |
| Package-level compiled regexps | In-function regexp.MustCompile | Package-level avoids recompiling on every call; matches MDS parser style |

**No new installation required** — all dependencies already in `go.mod`.

## Architecture Patterns

### Recommended Project Structure

```
internal/parser/brocade/
├── doc.go           # Already exists (package declaration only)
├── parser.go        # New: exported Parse(), detection, state machine
└── parser_test.go   # New: table-driven tests against testdata/brocade/

testdata/brocade/
├── .gitkeep         # Already exists
├── cfgshow_basic.cfg
├── cfgshow_continuation.cfg
├── cli_basic.cfg
├── cli_multizone.cfg
└── edge_cases.cfg
```

### Pattern 1: Exported Parse Entry Point (mirrors MDS parser)

**What:** A single `Parse(r io.Reader) (*ir.ZoningConfig, error)` function that reads all lines, detects format, delegates to sub-parsers, returns a fully populated `*ir.ZoningConfig`.
**When to use:** Always — this is the only public API for the package.
**Example:**

```go
// Source: mirrors internal/parser/mds/parser.go structure
func Parse(r io.Reader) (*ir.ZoningConfig, error) {
    scanner := bufio.NewScanner(r)
    var lines []string
    for scanner.Scan() {
        lines = append(lines, scanner.Text())
    }
    if err := scanner.Err(); err != nil {
        return nil, err
    }

    cfg := &ir.ZoningConfig{
        SourceFormat: "brocade-fos",
        Aliases:      make(map[string]*ir.Alias),
        Zones:        make(map[string]*ir.Zone),
        ZoneConfigs:  make(map[string]*ir.ZoneConfig),
    }

    if detectCLIFormat(lines) {
        parseCLIFormat(lines, cfg)
    } else {
        parseCfgshowFormat(lines, cfg)
    }
    return cfg, nil
}
```

### Pattern 2: Format Auto-Detection (PARSE-09)

**What:** Scan the first N lines for mutually exclusive markers. cfgshow always has `Defined configuration:` as a top-level header; CLI scripts always start with `alicreate`, `zonecreate`, or `cfgcreate` verbs.
**When to use:** Called from `Parse()` before delegating to format-specific parser.
**Example:**

```go
// Source: derived from format specification in phase description
func detectCLIFormat(lines []string) bool {
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        if trimmed == "Defined configuration:" {
            return false // cfgshow format
        }
        if reCLICommand.MatchString(trimmed) {
            return true // CLI format
        }
    }
    return false // default to cfgshow if ambiguous
}

// Package-level regex for detection
var reCLICommand = regexp.MustCompile(`^(alicreate|zonecreate|cfgcreate)\s+`)
```

### Pattern 3: cfgshow State Machine (PARSE-07)

**What:** Line-by-line state machine that tracks whether we are inside a cfg, zone, or alias block. The `Defined configuration:` header starts the parse. Type tokens (`cfg:`, `zone:`, `alias:`) switch state. Lines with only whitespace + member data accumulate members. Backslash continuation is handled by checking if a trimmed member line ends with `\`.
**When to use:** When `detectCLIFormat` returns false.

**cfgshow structure rules:**

- `Defined configuration:` — section header, start parse
- `cfg:   <name>` — switch state to cfg, store name
- `zone:  <name>` — switch state to zone, store name
- `alias: <name>` — switch state to alias, store name
- Lines after a type-token line contain member lists: whitespace-padded, semicolon-separated
- A line ending with `\` (after trimming) means the next line continues the member list
- `Effective configuration:` or end-of-file signals end of Defined configuration section

```go
// Source: derived from Brocade FOS cfgshow format specification
var (
    reCfgshowSection = regexp.MustCompile(`^Defined configuration:\s*$`)
    reCfgshowEffective = regexp.MustCompile(`^Effective configuration:\s*$`)
    reCfgToken   = regexp.MustCompile(`^\s+cfg:\s+(\S+)`)
    reZoneToken  = regexp.MustCompile(`^\s+zone:\s+(\S+)`)
    reAliasToken = regexp.MustCompile(`^\s+alias:\s+(\S+)`)
)

// Member line parsing: strip leading/trailing whitespace,
// strip trailing backslash, split on semicolons, trim each part
func parseMemberLine(line string) (members []string, continues bool) {
    trimmed := strings.TrimSpace(line)
    if strings.HasSuffix(trimmed, `\`) {
        continues = true
        trimmed = strings.TrimSuffix(trimmed, `\`)
    }
    for _, part := range strings.Split(trimmed, ";") {
        part = strings.TrimSpace(part)
        if part != "" {
            members = append(members, part)
        }
    }
    return members, continues
}
```

### Pattern 4: CLI Format Parser (PARSE-08)

**What:** Each command is a single line. Three command types:

- `alicreate "name", "pwwn"` — one alias per line, one member (pWWN only)
- `zonecreate "name", "member1;member2;..."` — one zone per line, semicolon-separated members
- `cfgcreate "name", "zone1;zone2;..."` — one cfg per line, semicolon-separated zone names

Members in zonecreate may be alias names or pWWNs (if the member contains `:`, treat as pWWN; otherwise treat as alias reference).

```go
// Source: derived from Brocade FOS CLI format specification
var (
    reAliCreate  = regexp.MustCompile(`^alicreate\s+"([^"]+)"\s*,\s*"([^"]+)"`)
    reZoneCreate = regexp.MustCompile(`^zonecreate\s+"([^"]+)"\s*,\s*"([^"]+)"`)
    reCfgCreate  = regexp.MustCompile(`^cfgcreate\s+"([^"]+)"\s*,\s*"([^"]+)"`)
)

func parseCLIFormat(lines []string, cfg *ir.ZoningConfig) {
    for _, line := range lines {
        line = strings.TrimSpace(line)
        if m := reAliCreate.FindStringSubmatch(line); m != nil {
            name, pwwn := m[1], normalizeWWN(m[2])
            cfg.Aliases[name] = &ir.Alias{Name: name, PWWN: pwwn, VSAN: 0}
            continue
        }
        if m := reZoneCreate.FindStringSubmatch(line); m != nil {
            name := m[1]
            z := &ir.Zone{Name: name, VSAN: 0}
            for _, member := range strings.Split(m[2], ";") {
                member = strings.TrimSpace(member)
                if member == "" { continue }
                if looksLikeWWN(member) {
                    z.Members = append(z.Members, &ir.ZoneMember{Type: "pwwn", Value: normalizeWWN(member)})
                } else {
                    z.Members = append(z.Members, &ir.ZoneMember{Type: "alias", Value: member})
                }
            }
            cfg.Zones[name] = z
            continue
        }
        if m := reCfgCreate.FindStringSubmatch(line); m != nil {
            name := m[1]
            zc := &ir.ZoneConfig{Name: name, VSAN: 0}
            for _, zoneName := range strings.Split(m[2], ";") {
                zoneName = strings.TrimSpace(zoneName)
                if zoneName != "" {
                    zc.ZoneNames = append(zc.ZoneNames, zoneName)
                }
            }
            cfg.ZoneConfigs[name] = zc
            continue
        }
    }
}
```

### Pattern 5: Zone Keys for Brocade IR

**What:** The MDS parser uses composite keys `"name@vsanN"` for `cfg.Zones` to support multi-VSAN configs. Brocade has no VSAN concept, so VSAN is always 0. For consistency with the MDS parser pattern — and to avoid breaking emitter consumers who key lookup by name — use the plain name as the key (not `"name@vsan0"`).
**When to use:** All Brocade parser writes to `cfg.Zones` and `cfg.ZoneConfigs`.

**Critical IR contract:** Looking at `zoningconfig.go` comments: "For Brocade (single-fabric, no VSANs), all zones use VSAN 0 as a sentinel." The `Zone.VSAN` and `ZoneConfig.VSAN` fields are set to 0. The map key should be the plain name since there is no ambiguity without VSAN scoping.

### Pattern 6: Shared normalizeWWN Helper

**What:** Both MDS and Brocade parsers need WWN normalization. The MDS parser defines `normalizeWWN` as a package-private function in its own package. The Brocade parser must define its own copy (or a shared `internal/ir` function — but since IR has zero imports by design, keep the helper local to each parser package).
**When to use:** Whenever a pWWN string is stored into `ir.Alias.PWWN` or `ir.ZoneMember.Value` for type "pwwn".

### Anti-Patterns to Avoid

- **Using "name@vsan0" as map key for Brocade zones:** Emitters and consumers in later phases will look up zones by name directly. Since Brocade is always VSAN 0, the `@vsan0` suffix adds complexity with no benefit. Use plain name.
- **Trying to parse cfgshow member lines as individual-line records:** Members in cfgshow appear as continuation lines after the type token, not as individual lines with keywords. They are semicolon-separated on a possibly multi-line block.
- **Treating backslash continuation as optional:** The `\` at end-of-line is mandatory for multi-line member blocks in cfgshow. Omitting the continuation check causes silent truncation.
- **Using HTML template instead of text/template:** Not applicable here (no template output in this phase), but flag for emitter phases.
- **Splitting on semicolons without trimming whitespace:** cfgshow member lines can have `member01; member02` (space after semicolon) or `member01;member02` (no space). Always `strings.TrimSpace` each split token.
- **Assuming CLI format alicreate has only one pWWN member:** In the CLI format specification, `alicreate "name", "pwwn"` has exactly one pWWN. Do not loop over semicolons for alicreate.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| WWN normalization | Custom wwn package | Local `normalizeWWN` function (copy from MDS parser) | Already working in MDS parser; no need for a shared package that adds import complexity |
| Member list parsing | Complex grammar parser | `strings.Split(memberBlock, ";")` + TrimSpace | Member separator is always semicolon; no nesting; regex split unnecessary |
| Format detection | Probabilistic heuristic | Exact header/verb matching | `Defined configuration:` and `alicreate` are mutually exclusive markers |
| Test fixtures | Generated from real switch | Hand-crafted minimal .cfg files | Keeps test complexity low; real configs may contain undocumented edge cases |

**Key insight:** Both Brocade formats are simple enough that stdlib string operations outperform regex for member splitting. Reserve regexp for line-type classification only.

## Common Pitfalls

### Pitfall 1: Backslash Continuation Silent Truncation

**What goes wrong:** Parser reads first line of a multi-line member block, registers members from that line, then starts a new block when it encounters the next indented line — missing all continuation members.
**Why it happens:** Parser doesn't check for trailing `\` before deciding the member block is complete.
**How to avoid:** After each member-data line, check `strings.HasSuffix(strings.TrimSpace(line), "\\")`. If true, set a `continuation` boolean and continue accumulating members on the next line without resetting state.
**Warning signs:** Test with `cfgshow_continuation.cfg` fixture — zone member count should match all members across the continuation, not just the first line.

### Pitfall 2: cfgshow Type Token vs Member Line Confusion

**What goes wrong:** Parser misidentifies a member line starting with whitespace as a type token (or vice versa), because both have leading whitespace in cfgshow output.
**Why it happens:** Type tokens like `cfg:   name` and member lines like `member01; member02` both start with spaces.
**How to avoid:** Use specific regexps that anchor on the type keyword: `^\s+cfg:\s+`, `^\s+zone:\s+`, `^\s+alias:\s+`. Member lines that don't match any type token are treated as member data.
**Warning signs:** A `cfg:` name getting confused with a member, or members getting skipped because they were mistaken for a new object.

### Pitfall 3: cfgshow Section Boundaries

**What goes wrong:** Parser processes lines outside the `Defined configuration:` section, potentially picking up junk from `Effective configuration:` or summary lines.
**Why it happens:** Some cfgshow outputs include an `Effective configuration:` section after the `Defined configuration:` section, with the same syntax. Processing both would duplicate entries.
**How to avoid:** Set a boolean `inDefinedSection` that becomes true on `Defined configuration:` and false on `Effective configuration:` or end-of-file. Only process type tokens and member lines when `inDefinedSection == true`.

### Pitfall 4: CLI Format — Missing Quotes or Comma Variants

**What goes wrong:** Regexp `^alicreate\s+"([^"]+)"\s*,\s*"([^"]+)"` fails to match a line where the name or member list has no quotes, or where there is no comma.
**Why it happens:** FOS CLI specification mandates quotes and comma, but real-world scripts or manual extracts may have variations (e.g., tabs instead of spaces, or a copy-paste with curly quotes).
**How to avoid:** Test with realistic fixture files. For v1, strict matching against the canonical format is correct — anything that doesn't match is silently skipped (same as MDS parser behavior for unrecognized lines). Document this in a test with a malformed line.

### Pitfall 5: Zone Map Key Strategy Inconsistency

**What goes wrong:** Using `"name@vsan0"` for Brocade zones creates a key strategy inconsistency with MDS zones (`"name@vsanN"`). Emitters in Phase 5/6 iterating `cfg.Zones` will need to know which key format was used.
**Why it happens:** The IR design comments say "For Brocade (single-fabric, no VSANs), all zones use VSAN 0 as a sentinel" — but doesn't specify the map key format.
**How to avoid:** Use plain name as map key for Brocade output (no `@vsan0` suffix). The `Zone.VSAN` field carries the 0 value for those who need to check it. This makes emitter iteration identical for both: iterate `cfg.Zones`, use `z.Name`, ignore VSAN for Brocade output.

### Pitfall 6: alicreate pWWN vs alias-name member type

**What goes wrong:** In `zonecreate` member lists, members can be either alias names (e.g., `host_01`) or raw pWWNs (e.g., `10:00:00:00:c9:ab:cd:ef`). If all members are treated as aliases, pWWN-only zones produce wrong IR.
**Why it happens:** The CLI format allows both — an alias reference or a direct pWWN.
**How to avoid:** Check if a member string looks like a WWN: contains `:` at regular intervals, length ~23 chars. A simple heuristic: `strings.Contains(member, ":")` is sufficient since valid FOS alias names cannot contain colons (they use `[A-Za-z0-9_$^-]` charset).

## Code Examples

Verified patterns from the project codebase:

### Scanner setup (mirrors MDS parser)

```go
// Source: internal/parser/mds/parser.go
scanner := bufio.NewScanner(r)
var lines []string
for scanner.Scan() {
    lines = append(lines, scanner.Text())
}
if err := scanner.Err(); err != nil {
    return nil, err
}
```

### normalizeWWN (copy from MDS parser)

```go
// Source: internal/parser/mds/parser.go
func normalizeWWN(raw string) string {
    compact := strings.ReplaceAll(raw, ":", "")
    compact = strings.ToLower(compact)
    if len(compact) != 16 {
        return strings.ToLower(raw)
    }
    parts := make([]string, 8)
    for i := range 8 {
        parts[i] = compact[i*2 : i*2+2]
    }
    return strings.Join(parts, ":")
}
```

### Test fixture loading pattern (mirrors MDS parser_test.go)

```go
// Source: internal/parser/mds/parser_test.go
fixturePath := filepath.Join("..", "..", "..", "testdata", "brocade", tt.fixture)
f, err := os.Open(fixturePath)
require.NoError(t, err, "failed to open fixture %s", tt.fixture)
defer f.Close() //nolint:errcheck

cfg, err := Parse(f)
require.NoError(t, err, "Parse must not return an error for %s", tt.fixture)
require.NotNil(t, cfg, "Parse must return non-nil cfg")
require.Equal(t, "brocade-fos", cfg.SourceFormat, "SourceFormat must be 'brocade-fos'")
```

### cfgshow backslash continuation handling

```go
// Source: derived from Brocade FOS format specification
func parseMemberLine(line string) (members []string, continues bool) {
    trimmed := strings.TrimSpace(line)
    if strings.HasSuffix(trimmed, `\`) {
        continues = true
        trimmed = strings.TrimSuffix(trimmed, `\`)
        trimmed = strings.TrimSpace(trimmed)
    }
    for _, part := range strings.Split(trimmed, ";") {
        part = strings.TrimSpace(part)
        if part != "" {
            members = append(members, part)
        }
    }
    return members, continues
}
```

### cfgshow state machine skeleton

```go
// Source: derived from Brocade FOS cfgshow format specification
const (
    stateIdle  = iota
    stateInCfgshowSection
    stateCfg
    stateZone
    stateAlias
)

func parseCfgshowFormat(lines []string, cfg *ir.ZoningConfig) {
    state := stateIdle
    var currentCfg *ir.ZoneConfig
    var currentZone *ir.Zone
    var currentAlias *ir.Alias
    continuation := false

    for _, line := range lines {
        if reCfgshowSection.MatchString(line) {
            state = stateInCfgshowSection
            continuation = false
            continue
        }
        if reCfgshowEffective.MatchString(line) {
            state = stateIdle // stop processing
            continue
        }
        if state == stateIdle {
            continue
        }

        // Type token detection
        if m := reCfgToken.FindStringSubmatch(line); m != nil && !continuation {
            name := m[1]
            zc := &ir.ZoneConfig{Name: name, VSAN: 0}
            cfg.ZoneConfigs[name] = zc
            currentCfg = zc
            currentZone = nil
            currentAlias = nil
            state = stateCfg
            continue
        }
        // ... similar for zone: and alias: tokens ...

        // Member data lines
        members, cont := parseMemberLine(line)
        continuation = cont
        switch state {
        case stateCfg:
            if currentCfg != nil {
                currentCfg.ZoneNames = append(currentCfg.ZoneNames, members...)
            }
        case stateZone:
            if currentZone != nil {
                for _, m := range members {
                    if looksLikeWWN(m) {
                        currentZone.Members = append(currentZone.Members,
                            &ir.ZoneMember{Type: "pwwn", Value: normalizeWWN(m)})
                    } else {
                        currentZone.Members = append(currentZone.Members,
                            &ir.ZoneMember{Type: "alias", Value: m})
                    }
                }
            }
        case stateAlias:
            if currentAlias != nil && len(members) > 0 && currentAlias.PWWN == "" {
                currentAlias.PWWN = normalizeWWN(members[0]) // alias has exactly one pWWN
            }
        }
    }
}
```

## Test Fixture Design

The following five fixtures cover all three requirements (PARSE-07, PARSE-08, PARSE-09):

### testdata/brocade/cfgshow_basic.cfg (PARSE-07, PARSE-09)

```
Defined configuration:
 cfg:   Production_cfg
                fabric_zone1; fabric_zone2
 zone:  fabric_zone1
                host_01; storage_01
 zone:  fabric_zone2
                host_02; storage_02
 alias: host_01
                10:00:00:00:c9:ab:cd:ef
 alias: storage_01
                50:05:07:61:01:23:45:67
 alias: host_02
                10:00:00:00:c9:ab:cd:ee
 alias: storage_02
                50:05:07:61:01:23:45:68
```

### testdata/brocade/cfgshow_continuation.cfg (PARSE-07 — backslash continuation)

```
Defined configuration:
 cfg:   BigFabric_cfg
                big_zone; small_zone
 zone:  big_zone
                member_01; member_02; \
                member_03; member_04
 zone:  small_zone
                only_member
 alias: member_01
                10:00:00:00:c9:00:00:01
 alias: member_02
                10:00:00:00:c9:00:00:02
 alias: member_03
                10:00:00:00:c9:00:00:03
 alias: member_04
                10:00:00:00:c9:00:00:04
 alias: only_member
                10:00:00:00:c9:00:00:05
```

### testdata/brocade/cli_basic.cfg (PARSE-08, PARSE-09)

```
alicreate "host_01", "10:00:00:00:c9:ab:cd:ef"
alicreate "storage_01", "50:05:07:61:01:23:45:67"
zonecreate "fabric_zone1", "host_01;storage_01"
cfgcreate "Production_cfg", "fabric_zone1"
```

### testdata/brocade/cli_pwwn_members.cfg (PARSE-08 — pWWN members in zonecreate)

```
zonecreate "direct_zone", "10:00:00:00:c9:ab:cd:ef;50:05:07:61:01:23:45:67"
cfgcreate "Direct_cfg", "direct_zone"
```

### testdata/brocade/edge_cases.cfg (empty zone, cfgshow + Effective section boundary)

```
Defined configuration:
 cfg:   Test_cfg
                empty_zone; real_zone
 zone:  empty_zone
 zone:  real_zone
                host_a
 alias: host_a
                10:00:00:00:c9:ff:ff:01

Effective configuration:
 cfg:   Test_cfg
                empty_zone; real_zone
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Brocade-specific confparser libraries | stdlib bufio+regexp (no third-party) | Project-level decision (CLAUDE.md) | No external FOS parsing library exists that's maintained; stdlib is the correct approach |
| Single-format parsers | Auto-detecting dual-format parser | Phase 3 requirement (PARSE-09) | Format detection must be invisible to the caller |

**No deprecated approaches:** This is new code with no legacy to migrate.

## Open Questions

1. **cfgshow alias: member type in zones**
   - What we know: cfgshow `zone:` member lines contain alias names or pWWNs (semicolon-separated); the format itself doesn't label them
   - What's unclear: Is there a canonical way to tell if a cfgshow zone member is an alias reference vs a raw pWWN at parse time, beyond the colon heuristic?
   - Recommendation: Use `strings.Contains(member, ":")` as the heuristic. Valid FOS alias names cannot contain colons (confirmed by FOS naming rules). This is HIGH confidence.

2. **cfgshow zone: member line containing a raw pWWN vs alias reference**
   - What we know: Both cfgshow and CLI format can have either pWWNs or alias references as zone members
   - What's unclear: Does cfgshow always resolve alias references to pWWNs in zone display, or does it show alias names?
   - Recommendation: Treat as alias if no colon; treat as pWWN if colon present. This matches the `ZoneMember.Type` IR design. MEDIUM confidence — based on FOS documentation review and format specification.

3. **Map key for Brocade zones (plain name vs name@vsan0)**
   - What we know: MDS parser uses composite key; IR comments say VSAN 0 is a sentinel for Brocade; emitters in Phase 5/6 haven't been written yet
   - What's unclear: Whether the emitters will iterate cfg.Zones by key format or by `z.VSAN` field
   - Recommendation: Use plain name as key for Brocade zones. Document this as a decision in STATE.md after Phase 3. LOW risk — can be changed before Phase 5 if emitter design differs.

## Environment Availability

Step 2.6: SKIPPED (no external dependencies — this is a pure Go code/testdata change. Go 1.26.1 is installed and verified. All libraries already in go.mod.)

**Verified runtime:**

- Go: 1.26.1 darwin/arm64
- `go build ./...`: SUCCESS
- `go test ./...`: 7 tests pass, 0 fail

## Validation Architecture

`workflow.nyquist_validation` is `false` in `.planning/config.json` — this section is skipped per specification.

## Sources

### Primary (HIGH confidence)

- `internal/parser/mds/parser.go` — reference implementation for parser structure, state machine pattern, normalizeWWN, io.Reader interface
- `internal/ir/zoningconfig.go` — canonical IR struct; VSAN 0 sentinel documented in comments
- `internal/parser/mds/parser_test.go` — reference for test structure, fixture path pattern, require usage
- `go.mod` — confirmed all required libraries already present; Go 1.26.1 in use
- `CLAUDE.md` (project) — confirmed: stdlib bufio+regexp, testify require (not assert), io.Writer/Reader interface pattern

### Secondary (MEDIUM confidence)

- Phase description in `<additional_context>` — cfgshow and CLI format examples are canonical; backslash continuation documented
- `.planning/REQUIREMENTS.md` — PARSE-07, PARSE-08, PARSE-09 requirements confirmed
- `.planning/ROADMAP.md` — Phase 3 goal and success criteria confirmed
- `.planning/phases/02-mds-parser/02-01-PLAN.md` — TDD plan structure reference (fixture-first → parser second)

### Tertiary (LOW confidence)

- FOS alias naming charset claim (`[A-Za-z0-9_$^-]` cannot contain colons) — drawn from CLAUDE.md technology stack section which was researched and confirmed in prior phases; used as basis for colon heuristic

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — all libraries already in project go.mod and in use by MDS parser
- Architecture: HIGH — directly mirrors MDS parser pattern with format-specific adaptations
- Pitfalls: HIGH — backslash continuation and cfgshow section boundaries are verifiable from format spec; zone key strategy is MEDIUM (emitter impact unknown until Phase 5)

**Research date:** 2026-03-29
**Valid until:** 2026-04-29 (stable domain — stdlib patterns don't change)

---

## Project Constraints (from CLAUDE.md)

Extracted actionable directives that the planner MUST enforce:

| Directive | Source | Constraint |
|-----------|--------|------------|
| Tech stack: Go only, single binary | CLAUDE.md § Constraints | No new runtime dependencies; all parsing in stdlib |
| Warn and continue | CLAUDE.md § Constraints | Parser never returns error for parse failures; appends to cfg.Warnings |
| Input is offline file | CLAUDE.md § Constraints | No live switch connection; parse from io.Reader only |
| Use bufio + regexp for parsing | CLAUDE.md § Recommended Stack | Confirmed for this phase |
| Use testify v1.11.1 with `require` | CLAUDE.md § Supporting Libraries | Use `require` (not `assert`) in all test assertions |
| Write all output to io.Writer | CLAUDE.md § Stack Patterns | Parser output is *ir.ZoningConfig, not text; pattern applies to emitter phases |
| io.Writer/io.Reader abstraction | CLAUDE.md § Stack Patterns | Parse(r io.Reader) signature is mandatory |
| golangci-lint v2 standard preset | CLAUDE.md § Dev Tools | Lint must pass; no new lint suppressions without justification |
| Use `require` not `assert` | CLAUDE.md § Supporting Libraries | `require` stops on first failure; correct for sequential parser assertions |
| No Viper, no logrus | CLAUDE.md § What NOT to Use | Forbidden in this project |
| GSD workflow enforcement | CLAUDE.md § GSD Workflow | All file changes go through GSD execute-phase command |
