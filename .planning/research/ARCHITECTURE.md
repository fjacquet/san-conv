# Architecture Research

**Domain:** Go CLI tool — bidirectional SAN zoning config converter (Cisco MDS NX-OS / Brocade FOS)
**Researched:** 2026-03-28
**Confidence:** HIGH

---

## Standard Architecture

The classical approach for config-format translators follows the same pipeline as compiler front-ends: each input format has a dedicated parser that produces a format-neutral intermediate representation (IR), and each output format has a dedicated emitter that consumes the IR. This structure is well-established in source-to-source translation research (CrossTL, ciscoconfparse) and directly applicable here.

### System Overview

```
┌─────────────────────────────────────────────────────────────────────┐
│                           CLI Entry Point                            │
│                     cmd/san-conv/main.go                             │
│           cobra root command + mds2brocade / brocade2mds            │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ reads flags, opens file, wires pipeline
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Parsers (frontend)                           │
│  ┌──────────────────────────┐   ┌──────────────────────────────┐    │
│  │  internal/parser/mds     │   │  internal/parser/brocade     │    │
│  │  NX-OS running-config    │   │  FOS cfgshow / script dump   │    │
│  │  → ZoningConfig IR       │   │  → ZoningConfig IR           │    │
│  └──────────────────────────┘   └──────────────────────────────┘    │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ ZoningConfig struct (canonical IR)
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Intermediate Representation                        │
│                     internal/ir/zoningconfig.go                      │
│                                                                      │
│   ZoningConfig                                                       │
│     ├── Aliases    []Alias      { Name, WWN }                        │
│     ├── Zones      []Zone       { Name, Members []ZoneMember }       │
│     │                  ZoneMember { Type: alias|pwwn, Value string } │
│     ├── Configs    []ZoneConfig { Name, ZoneNames []string }         │
│     └── Active     string       (active config/zoneset name)         │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ ZoningConfig struct
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Validators / Warnings                          │
│                  internal/validator/validator.go                     │
│   Name-length checks, character set issues, members with no alias,  │
│   WWN format normalization, empty zones — collected as []Warning     │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ ZoningConfig (possibly annotated) + []Warning
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Emitters (backend)                             │
│  ┌──────────────────────────┐   ┌──────────────────────────────┐    │
│  │  internal/emitter/mds    │   │  internal/emitter/brocade    │    │
│  │  ZoningConfig            │   │  ZoningConfig                │    │
│  │  → NX-OS CLI commands    │   │  → FOS CLI commands          │    │
│  │    (io.Writer)           │   │    (io.Writer)               │    │
│  └──────────────────────────┘   └──────────────────────────────┘    │
└──────────────────────────────┬──────────────────────────────────────┘
                               │ text to io.Writer (stdout or file)
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                            Output                                    │
│   stdout: FOS/NX-OS commands (paste-ready)                          │
│   stderr: warnings via slog (unconvertible constructs)              │
│   optional -o file.sh: executable script file                       │
└─────────────────────────────────────────────────────────────────────┘
```

### Component Responsibilities

| Component | Responsibility | Communicates With |
|-----------|----------------|-------------------|
| `cmd/san-conv/main.go` | Entry point; wires cobra, wires pipeline | cobra root, parser, emitter |
| `cmd/san-conv/cmd/root.go` | Root cobra command, persistent flags (`-o`, `--warn-only`) | cobra, subcommands |
| `cmd/san-conv/cmd/mds2brocade.go` | `mds2brocade` subcommand: opens input file, calls MDS parser, calls Brocade emitter | MDS parser, validator, Brocade emitter |
| `cmd/san-conv/cmd/brocade2mds.go` | `brocade2mds` subcommand: opens input file, calls Brocade parser, calls MDS emitter | Brocade parser, validator, MDS emitter |
| `internal/parser/mds` | Scans NX-OS running-config text, extracts device-alias DB + zones + zoneset, returns IR | bufio.Scanner, regexp, ir |
| `internal/parser/brocade` | Scans FOS cfgshow/script text, extracts aliases + zones + cfg, returns IR | bufio.Scanner, regexp, ir |
| `internal/ir` | Defines `ZoningConfig`, `Alias`, `Zone`, `ZoneMember`, `ZoneConfig` structs; zero logic | consumed by all packages |
| `internal/validator` | Checks IR for convertibility issues, emits `[]Warning`; never mutates IR | ir, slog |
| `internal/emitter/brocade` | Renders IR as FOS CLI commands via `text/template` to an `io.Writer` | ir, text/template |
| `internal/emitter/mds` | Renders IR as NX-OS CLI commands via `text/template` to an `io.Writer` | ir, text/template |

---

## Recommended Project Structure

```
san-conv/
├── cmd/
│   └── san-conv/
│       ├── main.go              # Minimal: cobra Execute()
│       └── cmd/
│           ├── root.go          # Root command, persistent flags
│           ├── mds2brocade.go   # mds2brocade subcommand
│           └── brocade2mds.go   # brocade2mds subcommand
├── internal/
│   ├── ir/
│   │   └── zoningconfig.go     # Canonical IR structs only — no logic
│   ├── parser/
│   │   ├── mds/
│   │   │   ├── parser.go        # MDS NX-OS parser
│   │   │   └── parser_test.go   # Table-driven tests with fixture files
│   │   └── brocade/
│   │       ├── parser.go        # Brocade FOS parser
│   │       └── parser_test.go   # Table-driven tests with fixture files
│   ├── validator/
│   │   ├── validator.go         # IR validation and warning collection
│   │   └── validator_test.go
│   └── emitter/
│       ├── brocade/
│       │   ├── emitter.go       # FOS CLI command emitter
│       │   ├── templates/
│       │   │   └── fos.tmpl     # text/template for FOS output
│       │   └── emitter_test.go
│       └── mds/
│           ├── emitter.go       # NX-OS CLI command emitter
│           ├── templates/
│           │   └── nxos.tmpl    # text/template for NX-OS output
│           └── emitter_test.go
├── testdata/
│   ├── mds/
│   │   ├── basic.conf           # Minimal MDS running-config fixture
│   │   ├── enhanced_mode.conf   # Device-alias enhanced-mode fixture
│   │   └── edge_cases.conf      # Long names, empty zones, comments
│   └── brocade/
│       ├── basic.cfgshow        # Minimal cfgshow output fixture
│       └── edge_cases.cfgshow   # Multi-line aliases, semicolons
├── go.mod
├── go.sum
└── .goreleaser.yml
```

### Structure Rationale

- **`internal/`:** Prevents external import of parsing/emitting packages. All logic is private to this binary — the `internal/` boundary is semantic, not just stylistic.
- **`internal/ir/`:** Keeping the IR struct definitions in a dedicated package with zero logic ensures no import cycles between parser and emitter packages.
- **`testdata/`:** Real fixture files at the top level (not embedded in test files) allow `go test` to use `os.Open(path)` and ops team to inspect the test corpus. Separate `mds/` and `brocade/` subdirs match the parser split.
- **`cmd/san-conv/cmd/`:** Separate files per subcommand following cobra-cli scaffolding convention. The `root.go` holds shared persistent flags; subcommand files hold only their own flags and run logic.

---

## Architectural Patterns

### Pattern 1: Parser State Machine with Line-by-Line Scanner

**What:** Each parser uses `bufio.Scanner` to read lines sequentially and maintains an explicit parser state (e.g., `stateIdle`, `stateInDeviceAliasDB`, `stateInZone`, `stateInZoneset`). State transitions are driven by `regexp.MustCompile` patterns matched against each trimmed line.

**When to use:** NX-OS config is hierarchical but line-oriented — indented sub-blocks under `device-alias database` or `zone name FOO vsan 1` headers. A state machine tracks which block is currently open. This is the standard pattern for Cisco-family config parsing and what ciscoconfparse uses internally (parent/child relationship model).

**Trade-offs:**
- Simple to reason about, no grammar definition needed
- Requires careful handling of block-exit conditions (blank lines, next top-level keyword)
- Regex maintenance grows proportionally to config complexity — acceptable here because the MDS zoning section uses a small, stable keyword set

**NX-OS key patterns:**

```go
// Section headers
var (
    reDeviceAliasDB  = regexp.MustCompile(`^device-alias database`)
    reDeviceAlias    = regexp.MustCompile(`^\s+device-alias name (\S+)\s+pwwn (\S+)`)
    reZoneName       = regexp.MustCompile(`^zone name (\S+) vsan (\d+)`)
    reMemberAlias    = regexp.MustCompile(`^\s+member device-alias (\S+)`)
    reMemberPWWN     = regexp.MustCompile(`^\s+member pwwn (\S+)`)
    reZonesetName    = regexp.MustCompile(`^zoneset name (\S+) vsan (\d+)`)
    reZonesetMember  = regexp.MustCompile(`^\s+member (\S+)`)
    reZonesetActivate= regexp.MustCompile(`^zoneset activate name (\S+) vsan (\d+)`)
)
```

**Brocade FOS key patterns (cfgshow format + CLI command format):**

```go
// cfgshow defined-config section
var (
    reDefinedCfg    = regexp.MustCompile(`^Defined configuration:`)
    reEffectiveCfg  = regexp.MustCompile(`^Effective configuration:`)
    reCfgLine       = regexp.MustCompile(`^\s*cfg:\s+(\S+)\s*(.*)`)
    reZoneLine      = regexp.MustCompile(`^\s*zone:\s+(\S+)\s*(.*)`)
    reAliasLine     = regexp.MustCompile(`^\s*alias:\s+(\S+)\s*(.*)`)
    reMembersCont   = regexp.MustCompile(`^\s+(.+)`)  // continuation lines
)

// FOS CLI command format (script input)
var (
    reAliCreate  = regexp.MustCompile(`^alicreate\s+"([^"]+)","([^"]+)"`)
    reZoneCreate = regexp.MustCompile(`^zonecreate\s+"([^"]+)","([^"]+)"`)
    reCfgCreate  = regexp.MustCompile(`^cfgcreate\s+"([^"]+)","([^"]+)"`)
    reCfgEnable  = regexp.MustCompile(`^cfgenable\s+"([^"]+)"`)
)
```

### Pattern 2: Format-Neutral Intermediate Representation (IR)

**What:** The IR struct captures the semantic content common to both formats, independent of syntax. The MDS parser and Brocade parser both produce the same `ZoningConfig` IR. The MDS emitter and Brocade emitter both consume the same `ZoningConfig` IR. Adding a third vendor requires only a new parser + emitter pair, not changes to the core.

**When to use:** Mandatory for any bidirectional converter with more than one source format. The CrossTL research paper demonstrates this reduces translation complexity from O(n²) to O(n) as formats are added.

**IR design rationale:**

```go
// internal/ir/zoningconfig.go

// ZoningConfig is the format-neutral canonical representation
// of all SAN zoning objects extracted from either MDS or Brocade config.
type ZoningConfig struct {
    Aliases  []Alias
    Zones    []Zone
    Configs  []ZoneConfig  // "zoneset" in MDS, "cfg" in Brocade
    Active   string        // Active zoneset/cfg name; empty if none found
}

type Alias struct {
    Name string  // MDS: device-alias name; Brocade: alicreate alias
    WWN  string  // normalized colon-separated lowercase, e.g. "50:06:01:65:3e:a0:1e:d7"
}

type Zone struct {
    Name    string
    Members []ZoneMember
}

type ZoneMember struct {
    Type  MemberType  // MemberTypeAlias | MemberTypePWWN
    Value string      // alias name or WWN string
}

type MemberType int

const (
    MemberTypeAlias MemberType = iota
    MemberTypePWWN
)

type ZoneConfig struct {
    Name      string
    ZoneNames []string
}
```

**Key IR design decisions:**
- WWNs stored normalized (lowercase, colon-separated) to canonicalize across format differences
- Zone members carry a type tag so emitters know whether to emit an alias reference or a raw WWN
- `Active` string captures the enabled zoneset/cfg name; Brocade requires `cfgenable` to be emitted; MDS requires `zoneset activate name X vsan Y`
- No VSAN information stored in v1 — VSAN-to-fabric mapping is explicitly out of scope

### Pattern 3: Template-Driven Emitters with `io.Writer` Interface

**What:** Emitters accept an `io.Writer` and a `*ir.ZoningConfig`, then render output through `text/template`. The `io.Writer` interface decouples emitters from output destination (stdout, file, test buffer).

**When to use:** Always for output generation. Direct `fmt.Fprintf` concatenation becomes unmaintainable once output has multi-line structure and conditional sections.

**Example — Brocade FOS emitter skeleton:**

```go
// internal/emitter/brocade/emitter.go

type Emitter struct {
    tmpl *template.Template
}

func New() (*Emitter, error) {
    tmpl, err := template.ParseFS(templateFS, "templates/fos.tmpl")
    if err != nil {
        return nil, fmt.Errorf("parse FOS template: %w", err)
    }
    return &Emitter{tmpl: tmpl}, nil
}

func (e *Emitter) Emit(w io.Writer, cfg *ir.ZoningConfig) error {
    return e.tmpl.Execute(w, cfg)
}
```

**Template output structure (fos.tmpl):**

```
{{- range .Aliases}}
alicreate "{{.Name}}","{{.WWN}}"
{{- end}}

{{- range .Zones}}
zonecreate "{{.Name}}","{{range $i, $m := .Members}}{{if $i}};{{end}}{{$m.Value}}{{end}}"
{{- end}}

{{- range .Configs}}
cfgcreate "{{.Name}}","{{join .ZoneNames ";"}}"
{{- end}}

{{- if .Active}}
cfgsave
cfgenable "{{.Active}}"
{{- end}}
```

### Pattern 4: Warn-and-Continue with Collected Diagnostics

**What:** The validator package (and optionally parsers) collect non-fatal issues into a `[]Warning` slice rather than returning an error. The command layer prints all warnings to stderr via `slog.Warn` after conversion completes, then exits 0 if output was produced.

**When to use:** Required by the project constraint "partial output is better than stopping mid-conversion." Ops teams need to see what converted and review warnings, not receive a hard stop.

**Example:**

```go
// internal/validator/validator.go

type Warning struct {
    Object  string  // "zone:PROD_ZONE_01" or "alias:host-hba0"
    Message string  // "name exceeds Brocade 64-char limit, will be truncated"
}

func Validate(cfg *ir.ZoningConfig, direction Direction) []Warning {
    var warnings []Warning
    for _, alias := range cfg.Aliases {
        if len(alias.Name) > 64 {
            warnings = append(warnings, Warning{
                Object:  "alias:" + alias.Name,
                Message: "name length " + strconv.Itoa(len(alias.Name)) + " exceeds Brocade 64-char limit",
            })
        }
        // check for dashes in Brocade direction (FOS 7.x ignores dashes in names)
        if direction == DirectionMDS2Brocade && strings.Contains(alias.Name, "-") {
            warnings = append(warnings, Warning{
                Object:  "alias:" + alias.Name,
                Message: "name contains dashes; FOS 7.x silently ignores dashes — consider replacing with underscores",
            })
        }
    }
    // ... zone member checks, empty zone checks
    return warnings
}
```

---

## Data Flow

### MDS to Brocade Conversion

```
User: san-conv mds2brocade --input mds.conf [--output fos.sh]
    ↓
cmd/mds2brocade.go
    ↓ os.Open(inputFile)
    ↓
internal/parser/mds.Parse(file io.Reader) (*ir.ZoningConfig, error)
    ↓ bufio.Scanner, state machine, regexp matching
    ↓ → ZoningConfig{Aliases, Zones, Configs, Active}
    ↓
internal/validator.Validate(cfg, DirectionMDS2Brocade) []Warning
    ↓ name checks, empty-zone checks, dash-in-name checks
    ↓ → []Warning (logged to stderr via slog; execution continues)
    ↓
internal/emitter/brocade.Emit(w io.Writer, cfg) error
    ↓ text/template renders FOS commands
    ↓
stdout (or file via -o):
    alicreate "host_hba0","50:01:11:..."
    alicreate "array_spa0","50:06:..."
    zonecreate "host_hba0_array_spa0","host_hba0;array_spa0"
    cfgcreate "PROD_FAB_A","host_hba0_array_spa0"
    cfgsave
    cfgenable "PROD_FAB_A"

stderr (slog.Warn for each Warning):
    WARN alias:host-hba0 name contains dashes; FOS 7.x silently ignores dashes
```

### Brocade to MDS Conversion

```
User: san-conv brocade2mds --input fos.cfgshow [--output mds_commands.txt]
    ↓
cmd/brocade2mds.go
    ↓ os.Open(inputFile)
    ↓
internal/parser/brocade.Parse(file io.Reader) (*ir.ZoningConfig, error)
    ↓ handles both cfgshow format and CLI script format
    ↓ → ZoningConfig{Aliases, Zones, Configs, Active}
    ↓
internal/validator.Validate(cfg, DirectionBrocade2MDS) []Warning
    ↓
internal/emitter/mds.Emit(w io.Writer, cfg) error
    ↓ text/template renders NX-OS commands
    ↓
stdout (or file):
    device-alias database
      device-alias name host_hba0 pwwn 50:01:11:...
      device-alias name array_spa0 pwwn 50:06:...
    device-alias commit
    zone name host_hba0_array_spa0 vsan 1
      member device-alias host_hba0
      member device-alias array_spa0
    zoneset name PROD_FAB_A vsan 1
      member host_hba0_array_spa0
    zoneset activate name PROD_FAB_A vsan 1
```

### Key Data Flows Summary

1. **Parse flow:** `io.Reader` → scanner lines → state machine → populate IR structs
2. **Validate flow:** IR structs → rule checks → `[]Warning` (no IR mutation)
3. **Emit flow:** IR structs → template data binding → text to `io.Writer`
4. **Warning flow:** `[]Warning` → slog.Warn lines to stderr (separate from conversion output on stdout)

---

## Format Reference

### NX-OS Running-Config — Zoning Sections

The MDS parser must handle two independent config sections in running-config order:

**Device-alias database section (always before zone config):**
```
device-alias database
  device-alias name host_hba0 pwwn 50:01:11:22:33:aa:bb:cc
  device-alias name array_spa0 pwwn 50:06:01:65:3e:a0:1e:d7
device-alias commit
```
Note: In enhanced mode (default since NX-OS 8.5(1)), zone members reference device-alias names natively. In basic mode, zone member `pwwn` lines appear in place of `device-alias` lines.

**Zone and zoneset sections (per-VSAN):**
```
zone name host_hba0_array_spa0 vsan 1
  member device-alias host_hba0
  member device-alias array_spa0

zoneset name PROD_FAB_A vsan 1
  member host_hba0_array_spa0

zoneset activate name PROD_FAB_A vsan 1
```

### Brocade FOS — cfgshow Format

The Brocade parser must handle the defined-configuration section (the parseable part):
```
Defined configuration:
 cfg:   PROD_FAB_A  host_hba0_array_spa0
 zone:  host_hba0_array_spa0
         host_hba0; array_spa0
 alias: host_hba0   50:01:11:22:33:aa:bb:cc
 alias: array_spa0  50:06:01:65:3e:a0:1e:d7; \
        50:06:01:65:3e:a0:1e:d8

Effective configuration:
 cfg:   PROD_FAB_A
 ...
```

Multi-line continuation is indicated by `\` at the end of a line; the parser must accumulate continuation lines.

### Critical Naming Differences

| Concept | MDS NX-OS | Brocade FOS | Conversion Note |
|---------|-----------|-------------|-----------------|
| Alias object | `device-alias name X pwwn Y` | `alicreate "X","Y"` | Direct mapping |
| Zone object | `zone name X vsan N` | `zonecreate "X","members"` | VSAN dropped in Brocade |
| Zone set / cfg | `zoneset name X vsan N` | `cfgcreate "X","zones"` | VSAN dropped in Brocade |
| Activate | `zoneset activate name X vsan N` | `cfgenable "X"` | Must also emit `cfgsave` |
| Member separator | One `member` line per member | Semicolon-delimited in one arg string | Flatten on emit |
| Name max length | 64 chars (MDS 9.x) | 64 chars (FOS) | Same, but validate |
| Dash in names | Allowed | FOS 7.x silently drops dashes | Warn; suggest underscore |
| WWN format | `50:01:11:22:33:aa:bb:cc` | `50:01:11:22:33:aa:bb:cc` | Same; normalize on parse |

---

## Suggested Build Order

Dependencies flow bottom-up. Build and test each layer before building the next.

| Phase | Component | Depends On | Milestone Goal |
|-------|-----------|------------|----------------|
| 1 | `internal/ir` | nothing | Define IR structs; no logic; no tests needed beyond compile |
| 2 | `internal/parser/mds` | `internal/ir` | Parse MDS config files; table-driven tests with fixture files |
| 3 | `internal/parser/brocade` | `internal/ir` | Parse Brocade cfgshow; table-driven tests with fixture files |
| 4 | `internal/validator` | `internal/ir` | Validate IR; emit []Warning; unit tests for each rule |
| 5 | `internal/emitter/brocade` | `internal/ir` | Render FOS commands; golden-file tests comparing template output |
| 6 | `internal/emitter/mds` | `internal/ir` | Render NX-OS commands; golden-file tests |
| 7 | `cmd/san-conv` | all above | Wire cobra CLI; integration tests using fixture configs end-to-end |

**Rationale for this order:** The IR definition must be stable before any parser or emitter is written — changing IR structs later cascades changes into all parsers and emitters. Parsers and validators can be built in parallel with emitters once IR is fixed, but parsers should be complete first so end-to-end integration tests can be written as soon as the emitter is done (rather than waiting for both).

---

## Anti-Patterns

### Anti-Pattern 1: Parsing Directly to Output String

**What people do:** Write a single function that reads the MDS config and outputs FOS commands in one pass, with no intermediate representation.

**Why it's wrong:** Makes the reverse direction (Brocade-to-MDS) require duplicating all logic. Makes unit testing impossible — you cannot test "did I parse correctly?" independently of "did I emit correctly?". Any change to either format requires touching a monolithic function.

**Do this instead:** Always parse to the IR, then emit from the IR. The two steps have different failure modes, different test strategies, and different change velocity.

### Anti-Pattern 2: Mutating IR in the Validator

**What people do:** Have the validator "fix" names that are too long or replace dashes with underscores automatically during validation.

**Why it's wrong:** Silent mutation hides data loss. Ops teams need to see what changed. The validator's job is to report problems, not fix them. If name normalization is desired later, it belongs in a dedicated normalization pass with explicit opts-in flag (e.g., `--normalize-names`), not silently in validation.

**Do this instead:** Validator only collects `[]Warning`. IR is read-only in the validator. Any normalization is a future deliberate feature, not a side effect.

### Anti-Pattern 3: Using Global State for Parser Context

**What people do:** Use package-level variables to track the current parser state (e.g., `var currentZone *ir.Zone`).

**Why it's wrong:** Makes parsers non-reentrant, breaks concurrent use (e.g., future parallel conversion of multiple files), and makes tests that run multiple parse calls unpredictably share state.

**Do this instead:** All parser state lives in a local `parserState` struct within the `Parse` function. Pass it as a value through the loop, never expose it outside the function.

### Anti-Pattern 4: Writing Output Directly to os.Stdout in Emitters

**What people do:** Call `fmt.Println(...)` directly inside emitter functions.

**Why it's wrong:** Makes emitter output untestable without capturing stdout. Prevents `-o file` flag support without a rewrite. Couples emitters to the OS.

**Do this instead:** All emitters accept `io.Writer`. The command layer passes `os.Stdout` or an `*os.File`. Tests pass `bytes.Buffer` or `strings.Builder`. No emitter ever calls `os.Stdout` directly.

### Anti-Pattern 5: Stopping on First Parse Warning

**What people do:** Return an error from the parser when encountering an unknown line or unconvertible construct.

**Why it's wrong:** Real-world MDS configs contain comments, features irrelevant to zoning (interfaces, features, aaa config), and constructs the converter doesn't need. Stopping on first unknown line makes the tool unusable on real configs.

**Do this instead:** Unknown lines are silently skipped at parse time. Only semantically unconvertible constructs (e.g., a zone member type that has no Brocade equivalent) produce a `Warning`. The parser only errors on genuine file I/O failure or completely unrecognizable file structure.

---

## Scaling Considerations

This is a CLI tool with no server component. "Scaling" means handling larger and more complex config files reliably.

| Scale | Config Size | Approach |
|-------|-------------|---------|
| Typical | < 1,000 zones, < 10,000 aliases | `bufio.Scanner` line-by-line; entire IR held in memory; no concern |
| Large fabric | 10,000+ zones, 100,000+ aliases | IR stays in memory (structs are small); only concern is regex compilation — use `regexp.MustCompile` at package init, not inside loops |
| Edge case | Config files with Windows CRLF line endings | `bufio.Scanner` handles `\r\n` transparently; no special handling needed |
| Edge case | Config with embedded comments (`!` lines in NX-OS) | State machine skips `!`-prefixed lines in idle state; no impact |

The only realistic performance concern is regex compilation inside hot loops — mitigated entirely by compiling all regexes to package-level `var` at init time (standard Go idiom).

---

## Integration Points

### External Services

None. This tool is intentionally offline — it reads files, produces text output. No network calls, no switch connectivity, no API clients.

### Internal Boundaries

| Boundary | Communication | Notes |
|----------|---------------|-------|
| `cmd` → `parser` | Direct function call: `mds.Parse(io.Reader)` → `(*ir.ZoningConfig, error)` | Parser returns structured error on I/O failure only |
| `cmd` → `validator` | Direct function call: `validator.Validate(cfg, direction)` → `[]Warning` | Never errors; always returns list (possibly empty) |
| `cmd` → `emitter` | Direct function call: `emitter.Emit(io.Writer, cfg)` → `error` | Error only on template execution failure (programmer error, not user error) |
| `parser` → `ir` | Import only — parser imports `ir` package for struct definitions | No back-reference |
| `emitter` → `ir` | Import only — emitter imports `ir` package for struct definitions | No back-reference |
| `validator` → `ir` | Import only — validator imports `ir` package for struct definitions | No back-reference |

The dependency graph is a strict DAG: `cmd` → `{parser, validator, emitter}` → `ir`. No cycles are possible with this structure.

---

## Sources

- [Go compiler architecture: lexer, parser, AST, IR](https://go.dev/src/cmd/compile/README) — canonical compiler pipeline reference, HIGH confidence
- [CrossTL: Universal Translator with Unified IR](https://arxiv.org/abs/2508.21256) — O(n²) → O(n) via unified IR, MEDIUM confidence (research paper, pattern is sound)
- [Princeton CS320 IR lecture notes](https://www.cs.princeton.edu/courses/archive/spr03/cs320/notes/IR-trans1.pdf) — IR/translator frontend-backend separation, HIGH confidence (academic)
- [ciscoconfparse architecture](https://github.com/mpenning/ciscoconfparse) — parent/child config line model, state-machine equivalent for IOS/NX-OS, MEDIUM confidence (open source project)
- [Brocade FOS cfgshow output format](https://techdocs.broadcom.com/us/en/fibre-channel-networking/fabric-os/fabric-os-commands/9-2-x/Fabric-OS-Commands/cfgShow.html) — exact cfgshow text format with defined/effective sections, HIGH confidence (official Broadcom docs)
- [Brocade FOS alicreate/zonecreate/cfgcreate syntax](https://belznotbells.com/zoning-your-brocade-switches-using-cli-step-by-step-when-java-just-wont-work/) — CLI command syntax examples, MEDIUM confidence (practitioner blog, consistent with official docs)
- [Brocade dash-in-name issue (FOS 7.x)](https://www.penguinpunk.net/blog/brocade-alias-and-zone-syntax-or-how-fos-is-a-love-hate-thing/) — FOS silently ignores dashes in names, MEDIUM confidence (practitioner blog, known issue)
- [Cisco MDS NX-OS device-alias enhanced mode](https://www.cisco.com/c/en/us/td/docs/dcn/mds9000/sw/9x/configuration/fabric/cisco-mds-9000-nx-os-fabric-configuration-guide-9x/distributing_device_alias_services.html) — enhanced mode default since 8.5(1), zone member storage format, HIGH confidence (official Cisco docs)
- [Cisco MDS zone/zoneset config format](https://www.cisco.com/en/US/docs/storage/san_switches/mds9000/sw/san-os/quick/guide/qcg_zones.html) — zone name / zoneset name / member syntax, HIGH confidence (official Cisco docs)
- [Go project structure — glukhov.org](https://www.glukhov.org/post/2025/12/go-project-structure/) — cmd/internal layout, MEDIUM confidence (practitioner blog, consistent with go.dev/doc/modules/layout)
- [Go modules layout](https://go.dev/doc/modules/layout) — official cmd/internal structure, HIGH confidence (official Go docs)
- [bufio.Scanner package](https://pkg.go.dev/bufio) — line-by-line scanning, HIGH confidence (official Go docs)
- [text/template package](https://pkg.go.dev/text/template) — template-driven output generation, HIGH confidence (official Go docs)
- [go-warnings/warnings](https://github.com/go-warnings/warnings) — warn-and-continue collector pattern, MEDIUM confidence (well-maintained Go package)
- [Cisco ZoneMigrator tool](https://github.com/Cisco-SAN/ZoneMigrator) — existing Cisco tool (Windows-only binary, no source); confirms input format is cfgshow output, MEDIUM confidence

---

*Architecture research for: Go CLI tool — SAN zoning config converter (Cisco MDS NX-OS / Brocade FOS)*
*Researched: 2026-03-28*
