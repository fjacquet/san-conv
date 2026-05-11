# Real-World MDS Config Robustness (Group A) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the MDS/Brocade config parsers robust to terminal-capture artifacts (`--More--` pager prompts, ANSI escapes, stray `\r`), add a `--vsan N` scoping flag with clear multi-VSAN diagnostics, and lock all of it down with tests — so real customer captures convert without silently losing zone members.

**Architecture:** A new `internal/preprocess` package turns a raw byte stream into clean logical lines; both parsers call it instead of running their own `bufio.Scanner` loop. The MDS parser gains richer multi-VSAN warnings. `converter.Run` gains an optional post-parse `filterVSAN` step driven by a new `--vsan` flag on the `mds2brocade` and root commands. Test coverage: unit tests for `preprocess`, parser tests proving artifact recovery and the new warnings, converter tests for VSAN filtering, plus a manual re-run of the four customer captures.

**Tech Stack:** Go 1.25 (stdlib `io`, `regexp`, `strings`, `sort`), `github.com/spf13/cobra` for flags, `github.com/stretchr/testify/require` for tests.

**Spec:** `docs/superpowers/specs/2026-05-11-real-world-config-robustness-design.md`

**Branch note:** The design-spec commit is already on branch `maincd`. Either keep working on `maincd` or branch from here (`git switch -c feat/real-world-config-robustness`) — your call; the plan does not assume a branch name.

**Deviation from the spec (intentional, minor):** The spec listed `pager_more.cfg` and `crlf.cfg` fixture files. Files containing raw ESC (`0x1b`) and `\r` (`0x0d`) bytes are hard to review and maintain. Instead, the terminal-artifact parser tests use inline Go strings with `\x1b`/`\r` escapes fed through `strings.NewReader` (the parsers already take an `io.Reader`). Plain-text fixtures (`doubled_device_alias.cfg`, `multi_vsan_collision.cfg`) are still real files. The `preprocess` unit tests already cover ANSI/CR handling exhaustively with inline strings.

**Conventions:** Conventional-commit prefixes (`feat:`, `fix:`, `test:`, `docs:`, `chore:`). Every commit message ends with the line `Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>`. Run commands from the repo root (`/Users/fjacquet/Projects/san-conv`).

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `internal/preprocess/preprocess.go` | **create** | `Clean(io.Reader) ([]string, error)` + private `cleanText(string) []string`: strip ANSI/VT100 CSI sequences, normalize `\r\n`/`\r` to `\n`, drop standalone `--More--` lines. |
| `internal/preprocess/preprocess_test.go` | **create** | Table-driven unit tests for `Clean`. |
| `internal/parser/mds/parser.go` | **modify** | Replace the `bufio.Scanner` loop in `Parse` with `preprocess.Clean`. Replace the `seenVSANs` logic in `pass2BuildZones` with per-VSAN counts + cross-VSAN name-collision detection; emit one summary warning at the end. |
| `internal/parser/mds/parser_test.go` | **modify** | Add `TestParse_TerminalArtifacts`; add table cases for `doubled_device_alias.cfg` and `multi_vsan_collision.cfg`; add a multi-VSAN-breakdown assertion for `multi_vsan.cfg`. |
| `internal/parser/brocade/parser.go` | **modify** | Replace the `bufio.Scanner` loop in `Parse` with `preprocess.Clean`. |
| `internal/parser/brocade/parser_test.go` | **modify** | Add `TestParse_TerminalArtifacts` proving an ANSI/`--More--`-noisy CLI script still parses. |
| `internal/converter/converter.go` | **modify** | Add `VSAN int` to `Options`; add `filterVSAN(*ir.ZoningConfig, int)`; call it after parsing when `Direction == "mds2brocade"` and `VSAN != 0`. |
| `internal/converter/converter_test.go` | **modify** | Add `TestRun_MDS2Brocade_VSANFilter` and `TestRun_MDS2Brocade_VSANFilterNoMatch`. |
| `cmd/mds2brocade.go` | **modify** | Add `--vsan` int flag (default 0); pass it into `converter.Options`. |
| `cmd/root.go` | **modify** | Add `--vsan` int flag (default 0) to the root command; pass it into `converter.Options`. |
| `testdata/mds/doubled_device_alias.cfg` | **create** | A `device-alias database` block listed twice + a zone — mirrors the `show zoneset active` + `show running-config` capture shape. |
| `testdata/mds/multi_vsan_collision.cfg` | **create** | The same zone name in VSAN 10 and VSAN 20 with different members. |
| `.gitignore` | **modify** | Ignore `customers/` (real customer WWNs/hostnames). |

---

## Task 1: `internal/preprocess` package + unit tests

**Files:**
- Create: `internal/preprocess/preprocess.go`
- Test: `internal/preprocess/preprocess_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/preprocess/preprocess_test.go`:

```go
package preprocess

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "more prompt glued to a device-alias line via CR",
			in:   "device-alias database\n\x1b[7m--More--\x1b[m\r  device-alias name X pwwn 10:00:00:00:c9:11:22:33\n",
			want: []string{"device-alias database", "  device-alias name X pwwn 10:00:00:00:c9:11:22:33"},
		},
		{
			name: "more prompt glued to a zone member line via CR",
			in:   "zone name Z vsan 10\n\x1b[7m--More--\x1b[m\r    member fcalias Y\n    member pwwn 50:06:0e:80:04:7c:00:01\n",
			want: []string{"zone name Z vsan 10", "    member fcalias Y", "    member pwwn 50:06:0e:80:04:7c:00:01"},
		},
		{
			name: "CRLF line endings normalized",
			in:   "line one\r\nline two\r\n",
			want: []string{"line one", "line two"},
		},
		{
			name: "assorted ANSI sequences stripped, text intact",
			in:   "\x1b[Khello \x1b[1;32mworld\x1b[0m done\n",
			want: []string{"hello world done"},
		},
		{
			name: "standalone pager prompts dropped",
			in:   "a\n--More--\nb\n --More-- \nc\n------ More ------\nd\n",
			want: []string{"a", "b", "c", "d"},
		},
		{
			name: "a line merely containing the word more is kept",
			in:   "  device-alias name moreStorage pwwn 10:00:00:00:c9:11:22:33\n",
			want: []string{"  device-alias name moreStorage pwwn 10:00:00:00:c9:11:22:33"},
		},
		{
			name: "empty input yields empty slice",
			in:   "",
			want: []string{},
		},
		{
			name: "input that is only pager prompts yields empty slice",
			in:   "--More--\n--More--\n",
			want: []string{},
		},
		{
			name: "artifact-free config returned unchanged",
			in:   "device-alias database\n  device-alias name X pwwn 10:00:00:00:c9:11:22:33\ndevice-alias commit\n",
			want: []string{"device-alias database", "  device-alias name X pwwn 10:00:00:00:c9:11:22:33", "device-alias commit"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Clean(strings.NewReader(tt.in))
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/preprocess/...`
Expected: build failure — `package preprocess` / `Clean` undefined (the package file doesn't exist yet).

- [ ] **Step 3: Write the implementation**

Create `internal/preprocess/preprocess.go`:

```go
// Package preprocess cleans raw, possibly terminal-captured configuration text
// before it is handed to a parser. SAN switch "show running-config" output is
// frequently captured from an interactive terminal session, so it carries
// pager prompts ("--More--"), ANSI/VT100 escape sequences, and stray carriage
// returns that glue config lines onto the prompt line. This package strips all
// of that, producing the clean line slice a parser expects.
package preprocess

import (
	"io"
	"regexp"
	"strings"
)

// ansiCSI matches an ANSI/VT100 CSI escape sequence: ESC '[' parameter-bytes
// (0x30-0x3F) intermediate-bytes (0x20-0x2F) final-byte (0x40-0x7E). This
// covers SGR colors ("\x1b[7m", "\x1b[0m"), erase-in-line ("\x1b[K"), cursor
// movement, etc. — everything observed in real captures and then some.
var ansiCSI = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]")

// pagerPrompt matches a line that is nothing but a terminal pager prompt:
// "--More--", " --More-- ", "------ More ------", case-insensitive. A line
// that merely contains the word "more" as part of real content does not match
// (the dashes are required on both sides).
var pagerPrompt = regexp.MustCompile(`(?i)^\s*-{2,}\s*more\s*-{2,}\s*$`)

// Clean reads all of r and returns it as cleaned logical lines, with no
// trailing newline on any element. It:
//   - strips ANSI/VT100 CSI escape sequences,
//   - normalizes "\r\n" and bare "\r" to line breaks (this un-glues lines that
//     a "--More--" prompt was prefixed onto),
//   - drops lines that are only a pager prompt.
//
// Blank lines are preserved (parsers already treat them as "continue current
// block"). The single trailing empty element produced when the input ends with
// a newline is dropped, matching bufio.Scanner's behavior. The returned slice
// is always non-nil. Clean returns an error only if reading r fails.
func Clean(r io.Reader) ([]string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return cleanText(string(raw)), nil
}

// cleanText is the pure core of Clean, operating on an in-memory string.
func cleanText(s string) []string {
	s = ansiCSI.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	rawLines := strings.Split(s, "\n")
	// strings.Split appends a trailing "" when s ends with "\n"; drop it so the
	// output matches what bufio.Scanner produced for the same input.
	if n := len(rawLines); n > 0 && rawLines[n-1] == "" {
		rawLines = rawLines[:n-1]
	}

	lines := make([]string, 0, len(rawLines))
	for _, ln := range rawLines {
		if pagerPrompt.MatchString(ln) {
			continue
		}
		lines = append(lines, ln)
	}
	return lines
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/preprocess/...`
Expected: `ok  github.com/fjacquet/san-conv/internal/preprocess`

- [ ] **Step 5: Format and vet**

Run: `gofmt -l internal/preprocess/ && go vet ./internal/preprocess/...`
Expected: no output from `gofmt` (already formatted), no output from `go vet`.

- [ ] **Step 6: Commit**

```bash
git add internal/preprocess/preprocess.go internal/preprocess/preprocess_test.go
git commit -m "$(cat <<'EOF'
feat(preprocess): add terminal-capture cleanup package

Clean() strips ANSI/VT100 escape sequences, normalizes CR to LF, and
drops standalone --More-- pager prompts — turning a raw terminal capture
into the clean line slice the parsers expect.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Wire `preprocess.Clean` into the MDS parser

**Files:**
- Modify: `internal/parser/mds/parser.go` (imports; `Parse`, lines ~40-48)

- [ ] **Step 1: Replace the import block**

In `internal/parser/mds/parser.go`, change the imports from:

```go
import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
)
```

to:

```go
import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/fjacquet/san-conv/internal/preprocess"
)
```

- [ ] **Step 2: Replace the scanner loop in `Parse`**

In `internal/parser/mds/parser.go`, in `func Parse`, replace:

```go
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
```

with:

```go
	lines, err := preprocess.Clean(r)
	if err != nil {
		return nil, err
	}
```

- [ ] **Step 3: Run the MDS parser tests**

Run: `go test ./internal/parser/mds/...`
Expected: `ok` — all existing tests still pass (clean fixtures are unchanged by `Clean`).

- [ ] **Step 4: Run the full test suite + vet**

Run: `go test ./... && go vet ./...`
Expected: all packages `ok`; no vet output (in particular, `bufio` is no longer imported-but-unused).

- [ ] **Step 5: Commit**

```bash
git add internal/parser/mds/parser.go
git commit -m "$(cat <<'EOF'
refactor(parser/mds): read input via preprocess.Clean

Replaces the bufio.Scanner loop so terminal-capture artifacts (ANSI
codes, stray CR, --More-- prompts) are stripped before parsing. No
behavior change for clean configs.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Wire `preprocess.Clean` into the Brocade parser

**Files:**
- Modify: `internal/parser/brocade/parser.go` (imports; `Parse`, lines ~45-55)

- [ ] **Step 1: Replace the import block**

In `internal/parser/brocade/parser.go`, change the imports from:

```go
import (
	"bufio"
	"io"
	"regexp"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
)
```

to:

```go
import (
	"io"
	"regexp"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/fjacquet/san-conv/internal/preprocess"
)
```

- [ ] **Step 2: Replace the scanner loop in `Parse`**

In `internal/parser/brocade/parser.go`, in `func Parse`, replace:

```go
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
```

with:

```go
	lines, err := preprocess.Clean(r)
	if err != nil {
		return nil, err
	}
```

- [ ] **Step 3: Add a terminal-artifacts test**

In `internal/parser/brocade/parser_test.go`, add this test function at the end of the file:

```go
func TestParse_TerminalArtifacts(t *testing.T) {
	t.Parallel()

	// A CLI-format Brocade script captured through a terminal: ANSI-wrapped
	// --More-- prompt glued (via CR) onto the line after it.
	const captured = "alicreate \"host_01\", \"10:00:00:00:c9:ab:cd:ef\"\r\n" +
		"\x1b[7m--More--\x1b[m\ralicreate \"storage_01\", \"50:05:07:61:01:23:45:67\"\r\n" +
		"zonecreate \"z1\", \"host_01;storage_01\"\r\n"

	cfg, err := Parse(strings.NewReader(captured))
	require.NoError(t, err)
	require.Len(t, cfg.Aliases, 2, "both aliases must survive the --More-- prompt")
	require.Equal(t, "10:00:00:00:c9:ab:cd:ef", cfg.Aliases["host_01"].PWWN)
	require.Equal(t, "50:05:07:61:01:23:45:67", cfg.Aliases["storage_01"].PWWN)
	require.Len(t, cfg.Zones, 1)
	require.Len(t, cfg.Zones["z1"].Members, 2)
}
```

If `internal/parser/brocade/parser_test.go` does not already import `strings`, add `"strings"` to its import block.

- [ ] **Step 4: Run the brocade parser tests**

Run: `go test ./internal/parser/brocade/...`
Expected: `ok` — existing tests pass and `TestParse_TerminalArtifacts` passes.

- [ ] **Step 5: Run the full test suite + vet**

Run: `go test ./... && go vet ./...`
Expected: all `ok`; no vet output.

- [ ] **Step 6: Commit**

```bash
git add internal/parser/brocade/parser.go internal/parser/brocade/parser_test.go
git commit -m "$(cat <<'EOF'
refactor(parser/brocade): read input via preprocess.Clean

Same terminal-capture cleanup as the MDS parser, with a regression test
for an ANSI/--More-- noisy CLI script.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: MDS parser tests for artifact recovery + doubled-DB fixture

**Files:**
- Create: `testdata/mds/doubled_device_alias.cfg`
- Modify: `internal/parser/mds/parser_test.go`

- [ ] **Step 1: Create the doubled-DB fixture**

Create `testdata/mds/doubled_device_alias.cfg` (plain text — the `device-alias database` block appears twice, mirroring a capture that contains both `show zoneset active` and `show running-config`):

```
device-alias database
  device-alias name Host-A pwwn 20:00:00:00:c9:12:34:56
  device-alias name Storage-A pwwn 50:06:01:65:3e:a0:1e:d7
device-alias commit

zone name App-Zone vsan 10
  member device-alias Host-A
  member device-alias Storage-A

zoneset name ZS-VSAN10 vsan 10
  member App-Zone

zoneset activate name ZS-VSAN10 vsan 10

device-alias database
  device-alias name Host-A pwwn 20:00:00:00:c9:12:34:56
  device-alias name Storage-A pwwn 50:06:01:65:3e:a0:1e:d7
device-alias commit
```

- [ ] **Step 2: Write the failing tests**

In `internal/parser/mds/parser_test.go`, add this test function at the end of the file:

```go
func TestParse_TerminalArtifacts(t *testing.T) {
	t.Parallel()

	// A capture where ANSI-wrapped --More-- prompts (followed by a bare CR)
	// are glued onto the line after them — once before a device-alias entry,
	// once before a zone member.
	const captured = "device-alias database\r\n" +
		"  device-alias name Host-A pwwn 20:00:00:00:c9:12:34:56\r\n" +
		"\x1b[7m--More--\x1b[m\r  device-alias name Storage-A pwwn 50:06:01:65:3e:a0:1e:d7\r\n" +
		"device-alias commit\r\n" +
		"\r\n" +
		"zone name App-Zone vsan 10\r\n" +
		"  member device-alias Host-A\r\n" +
		"\x1b[7m--More--\x1b[m\r  member device-alias Storage-A\r\n" +
		"\r\n" +
		"zoneset name ZS-VSAN10 vsan 10\r\n" +
		"  member App-Zone\r\n"

	cfg, err := Parse(strings.NewReader(captured))
	require.NoError(t, err)

	// The device-alias glued behind --More-- must survive.
	require.Len(t, cfg.Aliases, 2, "both device-aliases must be parsed")
	require.Equal(t, "50:06:01:65:3e:a0:1e:d7", cfg.Aliases["Storage-A"].PWWN)

	// The zone member glued behind --More-- must survive.
	zone, ok := cfg.Zones["App-Zone@vsan10"]
	require.True(t, ok, "zone key 'App-Zone@vsan10' must exist")
	require.Len(t, zone.Members, 2, "both zone members must be parsed")
	require.Equal(t, "alias", zone.Members[1].Type)
	require.Equal(t, "Storage-A", zone.Members[1].Value)
}

func TestParse_DoubledDeviceAliasDatabase(t *testing.T) {
	t.Parallel()

	f, err := os.Open(filepath.Join("..", "..", "..", "testdata", "mds", "doubled_device_alias.cfg"))
	require.NoError(t, err)
	defer f.Close()

	cfg, err := Parse(f)
	require.NoError(t, err)
	// The database is listed twice but the aliases are de-duplicated by name.
	require.Len(t, cfg.Aliases, 2, "duplicate device-alias entries must collapse to 2 unique aliases")
	require.Len(t, cfg.Zones, 1)
	require.Len(t, cfg.ZoneConfigs, 1)
}
```

NOTE: confirm the relative path to `testdata/` matches the existing tests in this file. The existing table test uses a `fixturePath` / `filepath.Join` helper — reuse whatever pattern is already there rather than the literal `filepath.Join("..", "..", "..", ...)` above if it differs. (Run `grep -n "testdata" internal/parser/mds/parser_test.go` first.)

- [ ] **Step 3: Run the tests to verify they pass**

Run: `go test ./internal/parser/mds/... -run 'TestParse_TerminalArtifacts|TestParse_DoubledDeviceAliasDatabase' -v`
Expected: both PASS. (They should pass immediately — Task 2 already wired in `preprocess.Clean`, and de-dup-by-name is pre-existing behavior. These tests lock that behavior in.)

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: all `ok`.

- [ ] **Step 5: Commit**

```bash
git add testdata/mds/doubled_device_alias.cfg internal/parser/mds/parser_test.go
git commit -m "$(cat <<'EOF'
test(parser/mds): cover --More--/ANSI recovery and doubled device-alias DB

Locks in that members glued onto pager prompts survive parsing, and that
a device-alias database listed twice (show zoneset active + show
running-config capture) collapses to unique aliases.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5: Multi-VSAN warning upgrades in the MDS parser

**Files:**
- Modify: `internal/parser/mds/parser.go` (imports — add `"sort"`; `pass2BuildZones`, ~lines 128-252)
- Create: `testdata/mds/multi_vsan_collision.cfg`
- Modify: `internal/parser/mds/parser_test.go`

- [ ] **Step 1: Add the `sort` import**

In `internal/parser/mds/parser.go`, update the import block to include `"sort"` (keep imports alphabetized within the stdlib group):

```go
import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/fjacquet/san-conv/internal/preprocess"
)
```

- [ ] **Step 2: Replace the VSAN tracking in `pass2BuildZones`**

In `internal/parser/mds/parser.go`, in `func pass2BuildZones`:

(a) Replace the declaration

```go
	seenVSANs := make(map[int]bool)
```

with

```go
	zoneCountByVSAN := make(map[int]int)
	zonesetCountByVSAN := make(map[int]int)
	zoneNameFirstVSAN := make(map[string]int)
	collisionWarned := make(map[string]bool)
```

(b) In the zone-header branch (the `if m := reZoneHeader.FindStringSubmatch(line); m != nil {` block), replace the existing VSAN-tracking sub-block

```go
			// Track VSANs for multi-VSAN warning
			if !seenVSANs[vsan] {
				seenVSANs[vsan] = true
				if len(seenVSANs) == 2 {
					cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
						"multi-VSAN config detected (%d VSANs) — zones are VSAN-scoped; all converted to single Brocade fabric",
						len(seenVSANs),
					))
				}
			}
```

with

```go
			zoneCountByVSAN[vsan]++
			if firstVSAN, seen := zoneNameFirstVSAN[name]; seen && firstVSAN != vsan && !collisionWarned[name] {
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
					"zone name %q appears in VSAN %d and VSAN %d — Brocade zone names are fabric-wide; the output will contain conflicting zonecreate lines for %q unless you scope with --vsan",
					name, firstVSAN, vsan, name,
				))
				collisionWarned[name] = true
			}
			if _, seen := zoneNameFirstVSAN[name]; !seen {
				zoneNameFirstVSAN[name] = vsan
			}
```

(c) In the zoneset-header branch (the `if m := reZonesetHeader.FindStringSubmatch(line); m != nil {` block), replace the existing VSAN-tracking sub-block

```go
			// Track VSANs for multi-VSAN warning
			if !seenVSANs[vsan] {
				seenVSANs[vsan] = true
				if len(seenVSANs) == 2 {
					cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
						"multi-VSAN config detected (%d VSANs) — zones are VSAN-scoped; all converted to single Brocade fabric",
						len(seenVSANs),
					))
				}
			}
```

with

```go
			zonesetCountByVSAN[vsan]++
```

(d) At the very end of `pass2BuildZones`, **after** the `for _, line := range lines { ... }` loop closes and before the function returns, append:

```go
	// Emit one multi-VSAN breakdown warning if the input spans more than one VSAN.
	vsanSet := make(map[int]struct{})
	for v := range zoneCountByVSAN {
		vsanSet[v] = struct{}{}
	}
	for v := range zonesetCountByVSAN {
		vsanSet[v] = struct{}{}
	}
	if len(vsanSet) > 1 {
		vsans := make([]int, 0, len(vsanSet))
		for v := range vsanSet {
			vsans = append(vsans, v)
		}
		sort.Ints(vsans)
		parts := make([]string, 0, len(vsans))
		for _, v := range vsans {
			parts = append(parts, fmt.Sprintf("VSAN %d (%d zones, %d zonesets)", v, zoneCountByVSAN[v], zonesetCountByVSAN[v]))
		}
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"multi-VSAN input: %s — all merged into one Brocade fabric; pass --vsan N to scope to one",
			strings.Join(parts, ", "),
		))
	}
```

NOTE: `pass2BuildZones` currently has no `return` statement (it ends after the loop). Add the block above as the last statements in the function body.

- [ ] **Step 3: Create the collision fixture**

Create `testdata/mds/multi_vsan_collision.cfg`:

```
device-alias database
  device-alias name Host-A pwwn 20:00:00:00:c9:12:34:56
  device-alias name Storage-A pwwn 50:06:01:65:3e:a0:1e:d7
  device-alias name Storage-B pwwn 50:06:01:65:3e:a0:1e:d8
device-alias commit

zone name Shared vsan 10
  member device-alias Host-A
  member device-alias Storage-A

zoneset name ZS-VSAN10 vsan 10
  member Shared

zoneset activate name ZS-VSAN10 vsan 10

zone name Shared vsan 20
  member device-alias Host-A
  member device-alias Storage-B

zoneset name ZS-VSAN20 vsan 20
  member Shared

zoneset activate name ZS-VSAN20 vsan 20
```

- [ ] **Step 4: Write the failing tests**

In `internal/parser/mds/parser_test.go`, add this test function at the end of the file:

```go
func TestParse_MultiVSANWarnings(t *testing.T) {
	t.Parallel()

	t.Run("breakdown warning on multi_vsan.cfg", func(t *testing.T) {
		t.Parallel()
		f, err := os.Open(filepath.Join("..", "..", "..", "testdata", "mds", "multi_vsan.cfg"))
		require.NoError(t, err)
		defer f.Close()

		cfg, err := Parse(f)
		require.NoError(t, err)
		require.True(t, containsSubstr(cfg.Warnings, "multi-VSAN input:"),
			"want a multi-VSAN breakdown warning, got: %v", cfg.Warnings)
		require.True(t, containsSubstr(cfg.Warnings, "VSAN 10 (1 zones, 1 zonesets)"),
			"want VSAN 10 counts in the breakdown, got: %v", cfg.Warnings)
		require.True(t, containsSubstr(cfg.Warnings, "VSAN 20 (1 zones, 1 zonesets)"),
			"want VSAN 20 counts in the breakdown, got: %v", cfg.Warnings)
	})

	t.Run("collision warning on multi_vsan_collision.cfg", func(t *testing.T) {
		t.Parallel()
		f, err := os.Open(filepath.Join("..", "..", "..", "testdata", "mds", "multi_vsan_collision.cfg"))
		require.NoError(t, err)
		defer f.Close()

		cfg, err := Parse(f)
		require.NoError(t, err)
		require.True(t, containsSubstr(cfg.Warnings, `zone name "Shared" appears in VSAN 10 and VSAN 20`),
			"want a cross-VSAN collision warning, got: %v", cfg.Warnings)
		// The collision is reported exactly once even though both VSANs define "Shared".
		count := 0
		for _, w := range cfg.Warnings {
			if strings.Contains(w, `zone name "Shared" appears in VSAN`) {
				count++
			}
		}
		require.Equal(t, 1, count, "collision must be warned exactly once")
		// Both VSAN-scoped zones still exist in the IR.
		_, ok10 := cfg.Zones["Shared@vsan10"]
		_, ok20 := cfg.Zones["Shared@vsan20"]
		require.True(t, ok10 && ok20, "both Shared@vsan10 and Shared@vsan20 must exist")
	})
}

// containsSubstr reports whether any element of ss contains sub.
func containsSubstr(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
```

NOTE: if `internal/parser/mds/parser_test.go` already defines a helper named `containsSubstr` (or similar), reuse it and drop the duplicate definition. Also confirm the `testdata` relative path matches the file's existing convention (see Task 4 Step 2 NOTE).

- [ ] **Step 5: Run the new tests**

Run: `go test ./internal/parser/mds/... -run 'TestParse_MultiVSANWarnings' -v`
Expected: both subtests PASS.

- [ ] **Step 6: Run the full suite + format + vet**

Run: `gofmt -l internal/parser/mds/ && go vet ./... && go test ./...`
Expected: no `gofmt` output, no `go vet` output, all packages `ok`. In particular the existing `TestParse` table case for `multi_vsan.cfg` still passes (it does not assert warning emptiness).

- [ ] **Step 7: Commit**

```bash
git add internal/parser/mds/parser.go internal/parser/mds/parser_test.go testdata/mds/multi_vsan_collision.cfg
git commit -m "$(cat <<'EOF'
feat(parser/mds): richer multi-VSAN diagnostics

Replace the terse "multi-VSAN config detected" line with a per-VSAN
breakdown (zone/zoneset counts) emitted once, and warn when the same
zone name appears in two VSANs (Brocade has a flat zone namespace).

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6: `--vsan` flag and `filterVSAN`

**Files:**
- Modify: `internal/converter/converter.go`
- Modify: `internal/converter/converter_test.go`
- Modify: `cmd/mds2brocade.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Add `VSAN` to `Options` and the `filterVSAN` call site**

In `internal/converter/converter.go`:

(a) Add a field to the `Options` struct:

```go
type Options struct {
	InputFile  string
	Direction  string
	OutputFile string
	ScriptFile string
	FOSVersion string
	VSAN       int // when non-zero, convert only this VSAN's zones/zonesets (mds2brocade only)
}
```

(b) In `func Run`, immediately after the `switch opts.Direction { ... }` block that sets `cfg` (i.e. right before `// Step 3: Sanitize ONLY for mds2brocade direction.`), insert:

```go
	// Step 2b: Optionally scope to a single VSAN (mds2brocade only).
	if opts.Direction == "mds2brocade" && opts.VSAN != 0 {
		filterVSAN(cfg, opts.VSAN)
	}
```

- [ ] **Step 2: Add the `filterVSAN` function**

At the end of `internal/converter/converter.go` (after `func Run`), add:

```go
// filterVSAN removes everything that does not belong to the given VSAN: zones
// and zonesets whose VSAN differs, and fcaliases whose VSAN is neither 0 nor
// vsan. Device-aliases (VSAN 0, fabric-wide) are always kept. If filtering
// leaves no zones, a warning is appended to cfg.Warnings.
func filterVSAN(cfg *ir.ZoningConfig, vsan int) {
	for key, z := range cfg.Zones {
		if z.VSAN != vsan {
			delete(cfg.Zones, key)
		}
	}
	for key, zc := range cfg.ZoneConfigs {
		if zc.VSAN != vsan {
			delete(cfg.ZoneConfigs, key)
		}
	}
	for name, a := range cfg.Aliases {
		if a.VSAN != 0 && a.VSAN != vsan {
			delete(cfg.Aliases, name)
		}
	}
	if len(cfg.Zones) == 0 {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"--vsan %d matched no zones in the input; check the VSAN number", vsan))
	}
}
```

(`ir` and `fmt` are already imported by `converter.go`.)

- [ ] **Step 3: Write the failing converter tests**

In `internal/converter/converter_test.go`, add:

```go
// VSAN filter: only the requested VSAN's zones appear in the output.
func TestRun_MDS2Brocade_VSANFilter(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/multi_vsan.cfg",
		Direction:  "mds2brocade",
		FOSVersion: "pre-8.1",
		VSAN:       10,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	require.Contains(t, out, "Zone-A", "VSAN 10 zone must be present")
	require.NotContains(t, out, "Zone-B", "VSAN 20 zone must be filtered out")
	require.NotContains(t, out, "ZS-VSAN20", "VSAN 20 zoneset must be filtered out")
}

// VSAN filter with no matching VSAN: warns and produces no zones, no error.
func TestRun_MDS2Brocade_VSANFilterNoMatch(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/multi_vsan.cfg",
		Direction:  "mds2brocade",
		FOSVersion: "pre-8.1",
		VSAN:       999,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "--vsan 999 matched no zones")
	require.NotContains(t, stdout.String(), "zonecreate")
}
```

- [ ] **Step 4: Run the new converter tests**

Run: `go test ./internal/converter/... -run 'TestRun_MDS2Brocade_VSANFilter' -v`
Expected: both PASS.

- [ ] **Step 5: Wire the flag into `cmd/mds2brocade.go`**

In `cmd/mds2brocade.go`:

(a) In `RunE`, after the existing `fosVersion, _ := cmd.Flags().GetString("fos-version")` line, add:

```go
		vsan, _ := cmd.Flags().GetInt("vsan")
```

and add `VSAN: vsan,` to the `converter.Options{ ... }` literal.

(b) In `func init()`, after the existing `--fos-version` flag registration, add:

```go
	mds2brocadeCmd.Flags().Int("vsan", 0, "target VSAN to convert; 0 = convert all VSANs into one fabric")
```

- [ ] **Step 6: Wire the flag into `cmd/root.go`**

In `cmd/root.go`:

(a) In `RunE`, after `fosVersion, _ := cmd.Flags().GetString("fos-version")`, add:

```go
		vsan, _ := cmd.Flags().GetInt("vsan")
```

and add `VSAN: vsan,` to the `converter.Options{ ... }` literal.

(b) In `func init()`, after the existing `--fos-version` root flag registration, add:

```go
	rootCmd.Flags().Int("vsan", 0, "target VSAN to convert; 0 = convert all VSANs into one fabric")
```

- [ ] **Step 7: Build, vet, full test suite**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: build succeeds, no vet output, all packages `ok`.

- [ ] **Step 8: Smoke-test the flag**

Run: `go run . mds2brocade --help`
Expected: the help text lists `--vsan int   target VSAN to convert; 0 = convert all VSANs into one fabric`.

- [ ] **Step 9: Commit**

```bash
git add internal/converter/converter.go internal/converter/converter_test.go cmd/mds2brocade.go cmd/root.go
git commit -m "$(cat <<'EOF'
feat(cli): add --vsan flag to scope conversion to one VSAN

mds2brocade and the root command accept --vsan N; converter.Run prunes
zones, zonesets, and fcaliases outside that VSAN after parsing
(device-aliases are fabric-wide and always kept). 0 keeps the existing
merge-all behavior.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7: `.gitignore` the `customers/` directory

**Files:**
- Modify: `.gitignore`

- [ ] **Step 1: Confirm nothing under `customers/` is tracked**

Run: `git ls-files customers/`
Expected: empty output. (If it lists anything, run `git rm -r --cached customers/` and include that in the commit — but at the time this plan was written, `customers/` is entirely untracked.)

- [ ] **Step 2: Append the ignore rule**

Append to `.gitignore` (keep a trailing newline):

```
# Customer config captures — contain real WWNs and hostnames, never commit
customers/
```

- [ ] **Step 3: Verify**

Run: `git status --porcelain`
Expected: `customers/` no longer appears as untracked; `.gitignore` shows as modified.
Run: `git check-ignore customers/F1D1.txt`
Expected: prints `customers/F1D1.txt` (it is now ignored).

- [ ] **Step 4: Commit**

```bash
git add .gitignore
git commit -m "$(cat <<'EOF'
chore: gitignore customers/ — holds real customer WWNs and hostnames

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8: Full quality gate + manual regression on the four customer captures

**Files:** none (verification only)

- [ ] **Step 1: Run the full quality gate**

Run: `make check`
(That is `fmt` + `vet` + `lint` + `test`. If `golangci-lint` is not available via `go tool`, run `make fmt vet test` and note the lint gap.)
Expected: all green.

- [ ] **Step 2: Re-run the four customer captures and capture the numbers**

Run:

```bash
go build -o san-conv .
for f in F1D1 F1D3 F2D2 F2D4; do
  echo "=== $f ==="
  ./san-conv mds2brocade --fos-version 8.1+ --output /tmp/b_$f.txt customers/$f.txt 2>/tmp/e_$f.txt
  echo "  total WARN:           $(grep -c WARN /tmp/e_$f.txt)"
  echo "  no-valid-FOS-members: $(grep -c 'no valid FOS members' /tmp/e_$f.txt)"
  echo "  $(grep Summary /tmp/e_$f.txt)"
  echo "  zonecreate emitted:   $(grep -c zonecreate /tmp/b_$f.txt)"
done
```

Baseline (recorded in the spec, pre-change):

| File | total WARN | "no valid FOS members" |
|---|---|---|
| F1D1 | 17 | 16 |
| F1D3 | 18 | 17 |
| F2D2 | 19 | 18 |
| F2D4 | 19 | 18 |

Expected after the change: the "no valid FOS members" counts drop substantially (those zones had their only/last member glued behind a `--More--` prompt and now parse correctly), and `zonecreate` emitted counts rise correspondingly. The new multi-VSAN breakdown warning replaces the old terse one (still 1 such warning per file, since each file has VSAN 10/20 + 11/21).

- [ ] **Step 3: Sanity-check one recovered zone**

Run: `grep -n 'ESX-04_GVAMAX12_OR1C1' /tmp/b_F1D1.txt`
Expected: a `zonecreate "ESX-04_GVAMAX12_OR1C1", "..."` line is now present (pre-change it was skipped with "no valid FOS members").

- [ ] **Step 4: Try the `--vsan` flag on a real capture**

Run: `./san-conv mds2brocade --fos-version 8.1+ --vsan 11 customers/F1D1.txt 2>&1 | tail -5`
Expected: a much smaller output (only the ~4 VSAN-11 zones / the `Config_FabricA_Vsan11` zoneset), plus the multi-VSAN-input breakdown warning and the Summary line. No error.

- [ ] **Step 5: Record results in the PR description**

When opening the PR, paste the before/after table from Step 2 and the recovered-zone example from Step 3. (No commit — this step is documentation for the reviewer.)

- [ ] **Step 6 (optional cleanup): drop the stray gofmt-only changes if still present**

If `git status` still shows `internal/parser/mds/parser.go` / `internal/emitter/mds/emitter.go` modified from before this plan started (cosmetic `iota` re-alignment from an auto-format hook), they will have been folded into Task 2/5 commits already. If any unrelated cosmetic diff remains uncommitted at the end, either commit it as `style: gofmt` or `git checkout --` it — don't leave the tree dirty.

---

## Self-Review

**Spec coverage:**
- Terminal-artifact preprocessing (ANSI / `\r` / `--More--`) → Task 1 (package), Tasks 2-3 (wired into both parsers), Task 4 (MDS recovery tests), Task 3 Step 3 (Brocade recovery test). ✔
- `--vsan N` flag + `filterVSAN` (zones/zonesets/fcaliases pruned, device-aliases kept, no-match warning) → Task 6. ✔
- Multi-VSAN per-VSAN breakdown warning (replaces terse line, fires once, handles 3+ VSANs) → Task 5. ✔
- Cross-VSAN zone-name-collision warning (once per name) → Task 5. ✔
- Synthetic test fixtures → `doubled_device_alias.cfg` (Task 4), `multi_vsan_collision.cfg` (Task 5); control-byte cases via inline strings (documented deviation). ✔
- `internal/preprocess` unit tests → Task 1. ✔
- Parser/converter test extensions → Tasks 3, 4, 5, 6. ✔
- Manual before/after re-run of the 4 customer files → Task 8. ✔
- `.gitignore customers/` → Task 7. ✔
- Non-goals (peer zoning, dedup of doubled blocks, backspace-erase) → not in any task, as intended. ✔
- Implementation order in the spec (preprocess → wire-in → fixtures/tests → multi-VSAN → `--vsan` → gitignore → manual run) → matches Tasks 1→8. ✔

**Placeholder scan:** No `TBD`/`TODO`/"handle edge cases"/"similar to Task N" — every code step shows complete code; every command shows expected output. Two `NOTE:` callouts ask the executor to confirm the `testdata` relative-path convention and to reuse an existing `containsSubstr` helper if one exists — these are verification instructions, not placeholders.

**Type consistency:** `Options.VSAN int` (Task 6) is read in Tasks 6 only. `filterVSAN(*ir.ZoningConfig, int)` defined and called in Task 6. `preprocess.Clean(io.Reader) ([]string, error)` defined in Task 1, called identically in Tasks 2 and 3. `cleanText(string) []string` private, used only inside the package. `containsSubstr([]string, string) bool` defined once in Task 5. Map names `zoneCountByVSAN` / `zonesetCountByVSAN` / `zoneNameFirstVSAN` / `collisionWarned` used consistently within Task 5. Warning string literals are reused verbatim between the implementation step and the test assertions (`"multi-VSAN input:"`, `zone name "Shared" appears in VSAN 10 and VSAN 20`, `--vsan 999 matched no zones`). ✔

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-11-real-world-config-robustness.md`. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration.
2. **Inline Execution** — execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints.

Which approach?
