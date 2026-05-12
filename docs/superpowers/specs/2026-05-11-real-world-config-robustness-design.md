# Design: Real-World MDS Config Robustness (Group A)

- **Date:** 2026-05-11
- **Status:** Approved (brainstorming) — pending implementation plan
- **Author:** Frederic Jacquet (with Claude)
- **Scope tag:** Group A. Peer-zone consolidation is Group B — a separate spec/plan cycle.

## Background

Four real customer MDS running-config captures (`customers/F1D1.txt`, `F1D3.txt`,
`F2D2.txt`, `F2D4.txt`) were run through `san-conv mds2brocade`. They surfaced
problems the existing test fixtures never exercised:

1. **Terminal pager artifacts cause silent data loss.** The files are interactive
   terminal captures (SecureCRT/PuTTY style), not clean `show running-config`
   output. Each contains ~200+ embedded `--More--` pager prompts wrapped in ANSI
   escape codes, immediately followed (on the *same* physical line, separated only
   by a bare `\r`) by the next config line:

   ```
   zone name ESX-04_GVAMAX12_OR1C1 vsan 10
   \x1b[7m--More--\x1b[m\r    member fcalias ESX04        ← this member is dropped
       member fcalias GVAMAX12_OR1C1
   ```

   The parser's member regexes are anchored at `^\s+member …`. A line beginning
   with `\x1b` (ESC, `0x1b`) does not match, so the config line glued to the
   `--More--` prompt is silently lost. Per file: ~205–218 pager prompts; 16–18
   zones end up with "no valid FOS members → skipped"; ~40 `member pwwn` lines per
   file would be recovered if the input were cleaned first. The project's own
   `CLAUDE.md` states the tool "must handle real-world MDS configs including edge
   cases" — terminal-pager noise is the most common real-world wart, and the tool
   currently mishandles it.

2. **Multi-VSAN configs are merged with only a terse warning.** Each sample is one
   large VSAN (~1457–1461 zones in VSAN 10 or 20) plus a small secondary VSAN
   (4 zones in VSAN 11 or 21, the `Config_Fabric*_Vsan*` zoneset). All of it is
   flattened into a single Brocade fabric and reported with one `WARN` line. The
   Brocade emitter keys output by zone *name* only, so a zone name present in two
   VSANs with different members would produce two conflicting `zonecreate` lines
   for the same name — currently undetected.

3. **No regression coverage.** Nothing in `testdata/` reproduces any of the above,
   so a future refactor could silently reintroduce the data loss.

4. **`customers/` is not git-ignored.** The real captures contain customer WWNs and
   hostnames and must not be committed.

A secondary observation (not addressed here — see Non-Goals): the capture files
contain the zoning data twice (`show zoneset active` *and* `show running-config`).
The parser keys zones/zonesets by `name@vsanN`, so the second occurrence overwrites
the first. Once the `--More--` corruption is fixed, "last definition wins" lands on
the `show running-config` version, which is the correct, authoritative form.

## Goals

- Parsing is robust to terminal-pager artifacts: ANSI/VT100 escape sequences, bare
  `\r`, and standalone `--More--` prompts no longer cause line loss.
- Multi-VSAN inputs can be scoped to a single VSAN via a `--vsan N` flag; when not
  scoped, the user gets a clear per-VSAN breakdown and a warning about any zone-name
  collision across VSANs.
- The four customer samples convert with materially fewer spurious "no valid FOS
  members" warnings; the before/after numbers are recorded in the PR description.
- Synthetic test fixtures + unit tests lock all of the above against regression.
- `customers/` is git-ignored.

## Non-Goals (YAGNI)

- **Peer zoning** (smart-zoning → `zonecreate --peerzone`, and peer-zone
  consolidation of flat zones). That is Group B, with its own spec.
- **De-duplicating the doubled `show zoneset active` + `show running-config`
  blocks.** "Last definition wins" already resolves to the correct form once
  `--More--` is fixed. No change.
- **Backspace-style pager erasure** (`--More--` followed by `\x08` runs). Not
  present in any sample. Documented as a known limitation; not handled.
- **Brocade Virtual Fabrics output** (one logical fabric per VSAN in a single run).
  The supported multi-VSAN workflow is "run once per VSAN with `--vsan N`".

## Architecture

```
file ──► converter.Run
            │
            ├─► preprocess.Clean(io.Reader) ──► []string   (NEW package)
            │        strip ANSI · CR→LF · drop --More--
            │
            ├─► mds/parser.Parse(lines)  /  brocade/parser.Parse(lines)
            │        (scanner loop replaced by the call above)
            │
            ├─► [if opts.VSAN != 0] filterVSAN(cfg, opts.VSAN)   (NEW helper, converter pkg)
            │
            ├─► validator.Sanitize                          (unchanged)
            └─► emitter.Emit                                 (unchanged)
```

### New: `internal/preprocess` package

Single responsibility: turn a raw, possibly terminal-captured byte stream into a
clean slice of logical config lines.

```go
package preprocess

// Clean reads all of r, removes terminal/pager noise, and returns the result as
// logical lines (no trailing newline on any element).
//   - ANSI/VT100 CSI escape sequences are stripped.
//   - "\r\n" and bare "\r" are normalized to line breaks (this un-glues lines
//     that a "--More--" prompt was prefixed onto).
//   - Lines that are nothing but a pager prompt (e.g. "--More--", " --More-- ")
//     are dropped.
// Blank lines are preserved (the parsers already tolerate them).
// Clean only returns an error on an I/O failure reading r.
func Clean(r io.Reader) ([]string, error)
```

Internally:
1. `io.ReadAll(r)`.
2. Strip ANSI: regex `\x1b\[[0-?]*[ -/]*[@-~]` (the CSI grammar — covers `[7m`,
   `[m`, `[K`, cursor moves, SGR colors). Applied to the whole buffer before line
   splitting.
3. Normalize endings: replace `\r\n` → `\n`, then remaining `\r` → `\n`.
4. Split on `\n`.
5. Drop lines matching (case-insensitive) `^\s*-{2,}\s*More\s*-{2,}\s*$`.
6. Return the slice.

A package-private pure helper `cleanText(s string) []string` does steps 2–5 so the
unit tests don't need an `io.Reader`.

### Changed: `internal/parser/mds/parser.go` and `internal/parser/brocade/parser.go`

Replace the existing collection loop

```go
scanner := bufio.NewScanner(r)
var lines []string
for scanner.Scan() { lines = append(lines, scanner.Text()) }
if err := scanner.Err(); err != nil { return nil, err }
```

with

```go
lines, err := preprocess.Clean(r)
if err != nil { return nil, err }
```

No other parser logic changes. (The `bufio` import is dropped where it becomes
unused.)

### Changed: `internal/parser/mds/parser.go` — multi-VSAN warnings

The MDS parser stays VSAN-*agnostic* (it does not know about `--vsan`). Two warning
upgrades, both driven by data it already tracks:

- **Per-VSAN breakdown.** Today it appends one line the moment a second VSAN is
  seen. Instead: accumulate `map[int]struct{ zones, zonesets int }` while parsing,
  and at the end of `pass2BuildZones`, if `len(map) > 1`, append one warning:
  `multi-VSAN input: VSAN 10 (1457 zones, 1 zoneset), VSAN 11 (4 zones, 1 zoneset) — all merged into one Brocade fabric; pass --vsan N to scope to one`
  (VSANs listed in ascending order).
- **Cross-VSAN name collision.** Maintain `seenZoneNameVSAN map[string]int`. When a
  `zone name X vsan V` is parsed and `X` was previously seen in a different VSAN
  `W`, append:
  `zone name "X" appears in VSAN W and VSAN V — Brocade zone names are fabric-wide; the output will contain conflicting zonecreate lines for "X" unless you scope with --vsan`
  (emit once per colliding name).

### Changed: `internal/converter/converter.go` — `--vsan` filtering

- Add `VSAN int` to `converter.Options` (0 = no filtering, the default).
- After `Parse` and before `Sanitize`, if `opts.VSAN != 0`, call a new
  package-private helper:

  ```go
  // filterVSAN keeps only zones, zonesets, and fcaliases that belong to the given
  // VSAN. Device-aliases (VSAN 0, fabric-wide) are always kept.
  func filterVSAN(cfg *ir.ZoningConfig, vsan int)
  ```

  - `cfg.Zones`: drop entries whose `Zone.VSAN != vsan`.
  - `cfg.ZoneConfigs`: drop entries whose `ZoneConfig.VSAN != vsan`.
  - `cfg.Aliases`: drop entries whose `Alias.VSAN != 0 && Alias.VSAN != vsan`
    (device-aliases have `VSAN == 0`; fcaliases carry their VSAN).
  - If filtering leaves zero zones, append a warning:
    `--vsan %d matched no zones; check the VSAN number`.

### Changed: `cmd/mds2brocade.go` and `cmd/root.go`

- Add `--vsan` (int, default `0`) to `mds2brocadeCmd.Flags()` and to the root
  command's flags, mirroring how `--fos-version` is wired.
- Read it via `cmd.Flags().GetInt("vsan")` and pass it into `converter.Options`.
- `brocade2mds` does not get the flag (out of scope; the reverse direction has no
  VSAN concept on input).
- Help text: `target VSAN to convert; 0 = convert all VSANs into one fabric`.

### Changed: `.gitignore`

Add:

```
# Customer config captures — contain real WWNs and hostnames
customers/
```

(If any `customers/` content is already tracked, that is handled in the
implementation plan, not here — at the time of writing it is untracked.)

## Data Flow

1. `cmd/*` parses flags → `converter.Options{InputFile, Direction, OutputFile, ScriptFile, FOSVersion, VSAN}`.
2. `converter.Run` opens the file, hands the `*os.File` to the direction-appropriate
   `Parse`.
3. `Parse` calls `preprocess.Clean(r)` → `[]string` of clean logical lines, then
   runs its existing two passes over those lines.
4. Back in `Run`: if `VSAN != 0`, `filterVSAN(cfg, VSAN)`.
5. `validator.Sanitize` (mds2brocade only) → `emitter.Emit` → stdout / `--output`
   file / `--script` file.
6. Warnings (including the new multi-VSAN ones) and the summary line go to stderr,
   unchanged in mechanism.

## Error Handling

Consistent with the existing "warn and continue" philosophy:

- `preprocess.Clean` returns an error only on an I/O read failure; that propagates
  out of `Parse` as today's scanner error did. No new fatal paths.
- Malformed ANSI fragments, stray control bytes, an empty file, a file that is
  *only* pager prompts → produce an empty or near-empty line slice; the parser
  yields an empty `ZoningConfig`; the emitter writes empty sections; the summary
  line reports zeros. No panic, no error.
- `--vsan N` with no matching zones → warning, empty output sections, exit 0
  (matches "partial output is better than stopping").
- A negative `--vsan` value → treated as "no match" (warning), not an error.

## Testing

**New: `internal/preprocess/preprocess_test.go`** — table-driven unit tests:
- `\x1b[7m--More--\x1b[m\r  device-alias name X pwwn ...` → two lines: `--More--`
  is dropped, `  device-alias name X pwwn ...` is preserved verbatim.
- `\x1b[7m--More--\x1b[m\r    member fcalias Y` → `    member fcalias Y` preserved.
- CRLF-only input → CRs removed, content intact.
- `\x1b[K`, `\x1b[1;2H`, SGR color codes → stripped, surrounding text intact.
- Lines `--More--`, ` --More-- `, `------ More ------` → dropped; a line that
  merely *contains* `more` as part of a real token → kept.
- Empty input → empty slice. Input that is only pager prompts → empty slice.
- A clean, artifact-free config → returned unchanged (idempotence).

**New: `testdata/mds/` fixtures** (small, hand-written, no real data):
- `pager_more.cfg` — a `device-alias database` block and a `zone`/`zoneset` block
  with several lines glued behind `\x1b[7m--More--\x1b[m\r`, plus a couple of
  standalone `--More--` lines.
- `crlf.cfg` — a minimal valid config with `\r\n` line endings.
- `doubled_dbs.cfg` — the same `device-alias database` block listed twice (mirrors
  the `show zoneset active` + `show running-config` capture shape); asserts the
  alias count is the de-duplicated count, not double.
- `multi_vsan_collision.cfg` — one zone name defined in two VSANs with different
  members; asserts the collision warning fires.
  (The existing `testdata/mds/multi_vsan.cfg` is reused for the per-VSAN-breakdown
  assertion; extend it only if needed.)

**Extended: `internal/parser/mds/parser_test.go`**
- Parsing `pager_more.cfg` recovers every member/alias that was glued to a prompt.
- Parsing `multi_vsan_collision.cfg` produces the cross-VSAN collision warning.
- Parsing `multi_vsan.cfg` produces the per-VSAN-breakdown warning text.
- Parsing `doubled_dbs.cfg` yields the de-duplicated alias count.

**Extended: `internal/converter/converter_test.go`**
- `--vsan` set → only that VSAN's zones/zonesets appear in the IR / emitted output;
  device-aliases survive; fcaliases of other VSANs are dropped.
- `--vsan` with no match → warning present, output sections empty, no error.

**Extended: `internal/parser/brocade/parser_test.go`**
- A `cfgshow`/CLI fixture with ANSI + `--More--` noise parses correctly (the shared
  `preprocess.Clean` path applies to this parser too).

**Manual regression (recorded in the PR description, not committed):** re-run all
four `customers/*.txt` files through `mds2brocade` before and after; report the
change in total warnings and in "no valid FOS members" warnings per file.

**Existing tests:** all current tests in every package must still pass unchanged.

## Implementation Order (for the plan)

1. `internal/preprocess` package + its unit tests (TDD).
2. Wire `preprocess.Clean` into both parsers; confirm all existing parser/converter
   tests still pass.
3. Add the new `testdata/mds/` fixtures + the new parser tests for pager/CRLF/doubled.
4. Multi-VSAN warning upgrades in the MDS parser + tests.
5. `--vsan` flag: `Options` field, `filterVSAN` helper, cmd wiring + converter tests.
6. `.gitignore` entry.
7. Manual re-run of the four customer samples; capture before/after numbers.

## Open Questions

None — design approved 2026-05-11.

## Note on workflow

The project `CLAUDE.md` asks that code changes be routed through a GSD command.
This spec was produced via the `superpowers:brainstorming` flow at the user's
request; the implementation step will reconcile the two (e.g. run the plan under
`/gsd:execute-phase` or `/gsd:quick`) — to be confirmed with the user at the
brainstorming→implementation transition.
