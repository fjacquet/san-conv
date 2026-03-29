# Phase 4: Validator and Sanitizer - Research

**Researched:** 2026-03-29
**Domain:** Go string processing, regex-based sanitization, name collision detection
**Confidence:** HIGH

---

<phase_requirements>

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| SANI-01 | Enforce FOS 63-character name limit — truncate names exceeding it, emit warning with old and new names | `unicode/utf8` not needed; FOS names are ASCII-only by charset rule, so `len()` is correct. Truncate to `[:63]` after char-replacement to avoid cascade. |
| SANI-02 | Replace characters invalid in conservative FOS naming (`[A-Za-z0-9_]` pre-8.1, `[A-Za-z0-9_$^-]` FOS 8.1+) — warn per replacement | `regexp.MustCompile` on inverse charset; `regexp.ReplaceAllStringFunc` replaces each offending char with `_`; warn once per name (not per char). |
| SANI-03 | Detect when two or more names become identical after sanitization — emit collision warning with all affected originals, disambiguate output names | Two-pass scan: build `map[sanitized][]original`, then append `_2`/`_3` suffixes on any group with len > 1. |
</phase_requirements>

---

## Summary

Phase 4 implements a sanitizer that processes every name in the IR (`Alias.Name`, `Zone.Name`, `ZoneConfig.Name`) against Brocade FOS naming rules before any emitter runs. The sanitizer lives in `internal/validator/` — the package stub already exists with a doc comment explicitly describing this exact role.

The IR struct (`internal/ir/zoningconfig.go`) stores names as-is from parsing (pre-sanitization) and carries a `Warnings []string` slice for non-fatal diagnostics. The sanitizer must mutate the IR in-place (updating the `.Name` field of each struct plus the map keys that reference those names) and append to `cfg.Warnings`. This matches the existing parser convention: parsers also append to `cfg.Warnings` and never return errors for non-fatal issues.

The critical design insight from the doc comment is: "It reads IR and emits []Warning; it never mutates the IR." This is contradicted by the requirement to actually change names before the emitter runs. Research concludes the correct design is a sanitize-and-return pattern: `Sanitize(cfg *ir.ZoningConfig, fosVersion string) *ir.ZoningConfig` that rebuilds the maps with sanitized keys and updated `.Name` fields, appending warnings to the same `cfg.Warnings` slice. This avoids in-place key mutation problems on Go maps (deleting and reinserting while ranging is safe in separate passes but fragile).

**Primary recommendation:** Implement `internal/validator/sanitizer.go` with `Sanitize(*ir.ZoningConfig, string) *ir.ZoningConfig`. Apply operations in order: char-replacement → truncation → collision detection. Operate on all three name categories (Aliases, Zones, ZoneConfigs). Use `regexp.MustCompile` with inverse-charset patterns for char replacement.

---

## Project Constraints (from CLAUDE.md)

| Constraint | Source | Enforcement |
|------------|--------|-------------|
| Single Go binary, no runtime deps | CLAUDE.md Tech Stack | No new external packages; use stdlib `regexp`, `strings`, `fmt`, `sort` only |
| Warn and continue — partial output better than stopping | CLAUDE.md Constraints | Sanitizer must never return an error; append to `cfg.Warnings` and proceed |
| Input is offline static analysis only | CLAUDE.md Constraints | Not applicable to sanitizer (no I/O) |
| Write all output to `io.Writer` interface | CLAUDE.md Stack Patterns | Sanitizer does not write output — it mutates IR; this pattern applies to emitters |
| `ZoningConfig` is the canonical intermediate representation | CLAUDE.md Stack Patterns | Sanitizer receives `*ir.ZoningConfig`, returns `*ir.ZoningConfig` |
| Use `require` (not `assert`) in tests | CLAUDE.md Supporting Libraries | All test assertions use `require.Equal`, `require.Contains`, etc. |
| Table-driven tests | CLAUDE.md Stack Patterns | Sanitizer tests must use `tests []struct { name, ... }` pattern as in parser tests |
| No `logrus` — use `log/slog` | CLAUDE.md What NOT to Use | Sanitizer must not use logrus; warnings go to `cfg.Warnings []string`, not a logger |
| golangci-lint v2 `standard` preset | CLAUDE.md Dev Tools | `errcheck`, `govet`, `staticcheck`, `unused` enforced; all errors must be handled |
| No `html/template` | CLAUDE.md What NOT to Use | Not applicable (sanitizer has no template output) |
| IR package has zero imports | CLAUDE.md Decisions | Sanitizer imports `internal/ir`; IR package must not import `internal/validator` |

---

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| stdlib `regexp` | Go 1.26.1 (project go.mod) | Compile inverse-charset patterns, replace invalid chars | Already used in both parsers; no external dep; `MustCompile` at package level for zero-alloc hot path |
| stdlib `strings` | Go 1.26.1 | String manipulation (length checks, prefix detection) | Already used throughout parsers |
| stdlib `fmt` | Go 1.26.1 | Warning message formatting | Already used throughout parsers |
| stdlib `sort` | Go 1.26.1 | Deterministic ordering of collision groups in warnings | Required for reproducible output |
| `github.com/stretchr/testify/require` | v1.11.1 (go.mod) | Test assertions | Already in project; `require` sub-package per CLAUDE.md |

### No New Dependencies Required

All sanitizer functionality is achievable with stdlib. The existing `go.mod` already has testify for tests.

**Installation:** No new `go get` commands needed.

---

## Architecture Patterns

### Recommended Project Structure

```
internal/
└── validator/
    ├── doc.go              # Already exists — package declaration
    ├── sanitizer.go        # New: Sanitize() function + helpers
    └── sanitizer_test.go   # New: table-driven tests with inline IR construction
```

Test fixtures for the sanitizer should be built as inline `*ir.ZoningConfig` values in the test file (no `.cfg` fixture files needed — this is a pure Go-to-Go transformation, not a file parser).

### Test fixture location

Parser tests use `testdata/mds/` and `testdata/brocade/` directories. Sanitizer tests build IR directly in code. No new `testdata/` subdirectory is needed.

### Pattern 1: Package-level compiled regexes

**What:** Compile `regexp.MustCompile` at package init, not inside the function
**When to use:** Any hot-path string replacement
**Example:**

```go
// Source: internal/parser/mds/parser.go (established codebase pattern)
var (
    reInvalidCharsConservative = regexp.MustCompile(`[^A-Za-z0-9_]`)
    reInvalidCharsExtended     = regexp.MustCompile(`[^A-Za-z0-9_$^-]`)
)
```

### Pattern 2: Warning format matches existing parsers

**What:** Append plain strings to `cfg.Warnings` using `fmt.Sprintf`
**When to use:** Every non-fatal diagnostic
**Example:**

```go
// Source: internal/parser/mds/parser.go lines 150-153 (established pattern)
cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
    "IVR zone %q skipped — no FOS equivalent", ivrName,
))
```

Sanitizer warnings should use the same `%q`-quoted name style.

### Pattern 3: Rebuild maps with new keys

**What:** Range over original map, compute new key+name, insert into new map
**When to use:** Any time sanitized names diverge from original names (which changes map keys)
**Example (pseudocode):**

```go
// Safe pattern for map key mutation in Go
newAliases := make(map[string]*ir.Alias, len(cfg.Aliases))
for _, a := range cfg.Aliases {
    sanitized := sanitizeName(a.Name, fosVersion, cfg)
    a.Name = sanitized  // mutate struct field
    newAliases[sanitized] = a
}
cfg.Aliases = newAliases
```

This avoids the "ranging over a map while modifying it" anti-pattern. Build new map, assign back.

### Pattern 4: Operation order — char-replacement before truncation

**What:** Run char-replacement first, then truncate to 63 chars
**Why:** Truncation can expose a collision that only existed because of char replacement. Doing truncation first would also truncate names that might not need it if chars were replaced differently.
**Order:** char-replacement → truncation → collision detection
**Key insight from SANI-01/SANI-02 interaction:** Truncation can itself CAUSE collisions. Example: `VeryLongNameWith$` and `VeryLongNameWith#` both sanitize to `VeryLongNameWith_` then both truncate to the same 63-char string. Collision detection must run after both transformations are complete.

### Pattern 5: Collision disambiguation with suffix

**What:** When two names produce identical sanitized result, append `_2`, `_3`, etc.
**When to use:** After char-replacement and truncation pass finds duplicates
**Algorithm:**

```go
// Group originals by sanitized name
seen := make(map[string][]string) // sanitized → []original
for original, sanitized := range nameMap {
    seen[sanitized] = append(seen[sanitized], original)
}
// For groups with >1 original, disambiguate and warn
for sanitized, originals := range seen {
    if len(originals) > 1 {
        sort.Strings(originals) // deterministic suffix assignment
        for i, orig := range originals {
            if i == 0 { continue } // first keeps original sanitized name
            newName := fmt.Sprintf("%s_%d", sanitized, i+1)
            // check newName does not also collide (edge case: truncated suffix)
        }
        cfg.Warnings = append(cfg.Warnings, ...)
    }
}
```

The suffix must be applied BEFORE truncation length rechecks: if `sanitized` is 62 chars and suffix `_2` makes it 65, truncate the base before appending suffix.

### Pattern 6: FOS version constant

**What:** Use a typed constant or sentinel for FOS version, not bare string comparison
**When to use:** The `--fos-version` flag is not wired yet (Phase 7), but the sanitizer must accept it as a string parameter
**Example:**

```go
const (
    FOSVersionPre81    = "pre-8.1"
    FOSVersionExtended = "8.1+"
)
```

Or more simply: `fosExtended := fosVersion == "8.1+"` inside the function.

### Anti-Patterns to Avoid

- **In-place map key mutation:** Never `delete(cfg.Aliases, old); cfg.Aliases[new] = v` while ranging — build new map instead
- **Per-character warnings:** Do not warn once per replaced character — warn once per name that was changed, listing what changed (old→new)
- **Assuming names are already valid in Brocade IR:** Brocade cfgshow inputs may also contain names that violate FOS rules if the config was hand-edited. Sanitizer runs on ALL IR regardless of `SourceFormat`
- **Skipping Zone member alias references:** When a zone member has `Type: "alias"`, its `Value` is an alias name. If that alias name was sanitized (and its map key changed), the zone member's `Value` must also be updated to point to the new name. **This is the most subtle requirement of the phase.**

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Character classification | Custom char-by-char loop | `regexp.MustCompile` + `ReplaceAllStringFunc` | Regex handles the full charset in one call; custom loops miss edge cases |
| Deterministic collision output | Unsorted map iteration | `sort.Strings` before warning/disambiguation | Map iteration order in Go is random; tests would be flaky without explicit sort |
| String truncation | Manual rune counting | `name[:63]` byte slice | FOS names are ASCII-only by charset definition — no multi-byte rune concern |

**Key insight:** The complexity is in the interaction between operations (char-replace can shrink names; truncation can cause new collisions; zone member alias refs must track renames). The stdlib handles each transformation; the ordering logic is the custom work.

---

## IR Name Fields to Sanitize

This is the complete inventory of name fields and their relationships:

| Struct | Field | Map Key Pattern | References from |
|--------|-------|-----------------|-----------------|
| `ir.Alias` | `.Name` | `cfg.Aliases[name]` (MDS: plain name; Brocade: plain name) | `ZoneMember.Value` where `ZoneMember.Type == "alias"` |
| `ir.Zone` | `.Name` | `cfg.Zones[key]` where key is `name@vsanN` (MDS) or `name` (Brocade) | `ZoneConfig.ZoneNames[]` |
| `ir.ZoneConfig` | `.Name` | `cfg.ZoneConfigs[key]` same pattern as Zones | Not referenced by other IR fields |

**Critical:** When an `Alias.Name` changes, ALL `ZoneMember.Value` strings that referenced the old name must be updated. When a `Zone.Name` changes, ALL `ZoneConfig.ZoneNames` entries that referenced the old name must be updated.

The MDS parser uses composite keys (`name@vsanN`) for `cfg.Zones` and `cfg.ZoneConfigs`. The zone's `.Name` field holds the short name (without `@vsanN`). When sanitizing zone names, only the `.Name` part changes — the VSAN suffix must be preserved in the map key reconstruction.

---

## Common Pitfalls

### Pitfall 1: Map key vs. struct field divergence

**What goes wrong:** Sanitizing `a.Name` without also updating the map key leaves the IR in an inconsistent state — the struct says one name but the map key says another.
**Why it happens:** Go maps don't auto-update keys when values change.
**How to avoid:** Always rebuild the map: range original, compute new key/name, insert into fresh map, reassign.
**Warning signs:** Tests that look up by sanitized name fail with "key not found" even though the struct was renamed.

### Pitfall 2: Zone member alias references not updated

**What goes wrong:** Alias `A-host` is sanitized to `A_host`. Zone member `{Type: "alias", Value: "A-host"}` still points to the old name. Emitter looks up `cfg.Aliases["A_host"]` and finds it, but zone member still says `A-host` — emitter emits wrong name.
**Why it happens:** Zone members store alias names by value, not by pointer.
**How to avoid:** After building the alias rename map (`map[old]new`), do a second pass over all `Zone.Members` to update `.Value` where `.Type == "alias"`.
**Warning signs:** Emitter produces `alicreate "A-host"` (old name) in a zone while alias list shows `A_host`.

### Pitfall 3: MDS composite zone key reconstruction

**What goes wrong:** MDS zones use `"ZoneName@vsanN"` as map keys. Naive rename rebuilds the key as just `sanitizedName`, losing the `@vsanN` suffix.
**Why it happens:** The `.Name` field holds only the short name; the VSAN is in `.VSAN`.
**How to avoid:** When rebuilding the Zones map for MDS IR (`SourceFormat == "mds-nxos"`), reconstruct key as `fmt.Sprintf("%s@vsan%d", zone.Name, zone.VSAN)`. For Brocade IR (`SourceFormat == "brocade-fos"`), key is just `zone.Name`.
**Warning signs:** Tests looking up MDS zones by composite key fail after sanitization.

### Pitfall 4: Collision suffix itself exceeding 63 chars

**What goes wrong:** A name is exactly 63 chars after truncation. Appending `_2` makes it 65 chars. Output name is invalid.
**Why it happens:** Suffix is applied after the name is already at max length.
**How to avoid:** When disambiguating, compute `base = sanitized[:min(63-len(suffix), len(sanitized))]` then `newName = base + suffix`.
**Warning signs:** A collision-disambiguated name is longer than 63 chars.

### Pitfall 5: ZoneConfig zone name references not updated

**What goes wrong:** Zone `My-Zone` is sanitized to `My_Zone`. `ZoneConfig.ZoneNames` still contains `"My-Zone"`. Emitter tries to look up `cfg.Zones["My-Zone"]` and finds nothing.
**Why it happens:** Same reference-by-value problem as Pitfall 2, for zones.
**How to avoid:** After building zone rename map, do a second pass over all `ZoneConfig.ZoneNames` slices.

### Pitfall 6: Brocade source IR treated differently

**What goes wrong:** Brocade names often already conform to FOS naming rules (the switch enforced them). Sanitizing them might still change names if the config was hand-edited or sourced from a file with custom names.
**Why it happens:** Assumption that Brocade→MDS direction doesn't need sanitization.
**How to avoid:** Sanitizer runs unconditionally on all IR regardless of `SourceFormat`. The sanitizer is a FOS-output concern, but it runs any time the IR will eventually be emitted to FOS (i.e., always in mds2brocade, never needed for brocade2mds). The Phase 7 CLI wiring will call the sanitizer only in the mds2brocade path. For Phase 4, the sanitizer simply sanitizes whatever IR it receives.

---

## Code Examples

### Char replacement pattern (pre-8.1 mode)

```go
// Source: established codebase pattern — matches parser regex style in parser.go
var (
    reInvalidConservative = regexp.MustCompile(`[^A-Za-z0-9_]`)
    reInvalidExtended     = regexp.MustCompile(`[^A-Za-z0-9_$^-]`)
)

func replaceInvalidChars(name, fosVersion string) (string, bool) {
    re := reInvalidConservative
    if fosVersion == "8.1+" {
        re = reInvalidExtended
    }
    replaced := re.ReplaceAllString(name, "_")
    return replaced, replaced != name
}
```

### Warning format (matches existing parser warnings)

```go
// Source: internal/parser/mds/parser.go lines 259-263 (established pattern)
cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
    "alias %q renamed: invalid characters replaced %q -> %q",
    original, original, sanitized,
))
cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
    "alias %q truncated to 63 characters: %q -> %q",
    original, original, sanitized,
))
cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
    "collision: names %v all sanitize to %q — disambiguated",
    originals, sanitized,
))
```

### Table-driven test skeleton (matches parser test pattern)

```go
// Source: internal/parser/mds/parser_test.go (established test pattern)
func TestSanitize(t *testing.T) {
    t.Parallel()
    tests := []struct {
        name       string
        fosVersion string
        input      *ir.ZoningConfig
        checkFn    func(t *testing.T, out *ir.ZoningConfig)
    }{
        {
            name:       "alias name exceeding 63 chars is truncated with warning",
            fosVersion: "pre-8.1",
            input:      buildCfg(aliasWithName(strings.Repeat("A", 70))),
            checkFn: func(t *testing.T, out *ir.ZoningConfig) {
                t.Helper()
                expected := strings.Repeat("A", 63)
                require.Contains(t, out.Aliases, expected)
                require.Len(t, out.Warnings, 1)
                require.Contains(t, out.Warnings[0], "truncated")
            },
        },
        // ... more cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()
            out := Sanitize(tt.input, tt.fosVersion)
            tt.checkFn(t, out)
        })
    }
}
```

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Separate `sanitizer` and `validator` packages | Single `internal/validator` package (per doc.go) | Phase 1 decision | Both concerns live together; no cross-package import needed |
| Mutating IR in place | Rebuild maps with new keys | Phase 4 design | Avoids Go map-during-range unsafety |

**Deprecated/outdated:**

- doc.go says "It reads IR and emits []Warning; it never mutates the IR." This was written before the full sanitization requirements were understood. The plan must reconcile: the sanitizer DOES mutate `.Name` fields and rebuilds map keys. The warning-only interpretation is superseded by SANI-01/SANI-02/SANI-03.

---

## Open Questions

1. **Does the sanitizer mutate in-place or return a new IR?**
   - What we know: doc.go says "never mutates the IR"; SANI-01/02/03 require name changes
   - What's unclear: Whether the intent was "don't create a new ZoningConfig struct" or "don't change any field"
   - Recommendation: Mutate `.Name` fields and rebuild maps in-place on the same `*ir.ZoningConfig`. Allocate new maps; reassign `cfg.Aliases`, `cfg.Zones`, `cfg.ZoneConfigs`. Do not allocate a new `ZoningConfig`. This satisfies the spirit (no new allocation) while meeting the requirements.

2. **Should Brocade→MDS direction invoke the sanitizer?**
   - What we know: MDS naming rules differ from FOS rules; sanitizer is FOS-specific
   - What's unclear: Phase 4 is standalone; Phase 7 decides when to call it
   - Recommendation: Sanitizer is FOS-output-specific. Document in the function signature or a godoc comment that it should only be called for mds2brocade direction. Phase 7 enforces the call site.

3. **What happens when a collision suffix itself collides?**
   - What we know: `_2`, `_3` suffixes are the standard Go disambiguation pattern
   - What's unclear: If `Foo_2` already exists as an original name, the suffix creates a new collision
   - Recommendation: After appending a suffix, check if the new name already exists in the rename map. If so, increment the counter. Cap at a reasonable limit (e.g., 99) and emit a fatal-style warning if exceeded.

---

## Environment Availability

Step 2.6: SKIPPED — Phase 4 is pure Go code with no external dependencies beyond the existing project toolchain (Go 1.26.1, golangci-lint, testify). All tools confirmed working: `go test ./...` passes 13 tests across 9 packages.

---

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go testing stdlib + testify/require v1.11.1 |
| Config file | none (uses `go test` directly) |
| Quick run command | `go test ./internal/validator/...` |
| Full suite command | `go test ./...` |

### Phase Requirements to Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| SANI-01 | Name longer than 63 chars is truncated; warning emitted with old and new names | unit | `go test ./internal/validator/... -run TestSanitize/truncation` | No — Wave 0 |
| SANI-02 | Hyphen in pre-8.1 mode replaced with underscore; warning emitted | unit | `go test ./internal/validator/... -run TestSanitize/char_replacement` | No — Wave 0 |
| SANI-02 | Dollar and caret permitted in 8.1+ mode; no warning emitted | unit | `go test ./internal/validator/... -run TestSanitize/fos_version_extended` | No — Wave 0 |
| SANI-03 | Two names producing identical sanitized result; collision warning with all originals; disambiguated output | unit | `go test ./internal/validator/... -run TestSanitize/collision` | No — Wave 0 |
| SANI-01+03 | Truncation causing a collision between two formerly-distinct names | unit | `go test ./internal/validator/... -run TestSanitize/truncation_collision` | No — Wave 0 |

### Sampling Rate

- **Per task commit:** `go test ./internal/validator/...`
- **Per wave merge:** `go test ./...`
- **Phase gate:** `go test ./... && golangci-lint run ./internal/...` green before `/gsd:verify-work`

### Wave 0 Gaps

- [ ] `internal/validator/sanitizer_test.go` — covers all SANI-0x requirements
- [ ] `internal/validator/sanitizer.go` — the implementation itself (TDD: test file first)

*(No new test framework or fixtures directory needed — inline IR construction is sufficient)*

---

## Sources

### Primary (HIGH confidence)

- `internal/ir/zoningconfig.go` — IR struct fields, map key patterns, Warnings slice — read directly
- `internal/validator/doc.go` — package intent and placement — read directly
- `internal/parser/mds/parser.go` — warning format patterns, map rebuild patterns, regex conventions — read directly
- `internal/parser/brocade/parser.go` — additional pattern confirmation — read directly
- `internal/parser/mds/parser_test.go` — test structure and fixture patterns — read directly
- `CLAUDE.md` (project) — tech stack constraints, coding conventions — read directly
- `.planning/REQUIREMENTS.md` — SANI-01, SANI-02, SANI-03 specification — read directly
- `.planning/ROADMAP.md` — phase dependencies and success criteria — read directly
- `Makefile` — test/lint commands — read directly
- `.golangci.yml` — linter configuration (standard preset + misspell + gofmt) — read directly

### Secondary (MEDIUM confidence)

- Go stdlib `regexp` package documentation — `ReplaceAllString`, `MustCompile` API — HIGH confidence (stdlib, version-stable)
- Go stdlib `sort` package — `sort.Strings` for deterministic collision output — HIGH confidence

### Tertiary (LOW confidence)

- None — all findings are grounded in direct codebase reading or stdlib documentation

---

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — all libraries are stdlib or already in go.mod
- Architecture: HIGH — derived directly from existing codebase patterns
- Pitfalls: HIGH — derived from IR structure analysis and Go language properties
- Test patterns: HIGH — copied from working parser tests in the codebase

**Research date:** 2026-03-29
**Valid until:** Stable indefinitely (no external dependencies; all grounded in codebase)
