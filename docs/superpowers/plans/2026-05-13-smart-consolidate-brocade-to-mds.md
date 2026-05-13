# `--smart-consolidate`: flat Brocade zones → merged MDS smart zones Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `brocade2mds --smart-consolidate` that infers init/target roles from zone names + frequency, collapses flat single-initiator/single-target Brocade zones into per-target MDS smart zones (target as principal, hosts as init members), and emits `zone smart-zoning enable vsan N`.

**Architecture:** Mirror of `mds2brocade --peer-consolidate` (B2, ADR-0009). The existing `internal/consolidator` already handles the inference + merging; we generalize its zone-suffix naming so it works in both directions. The MDS emitter (B3, PR #5) already renders role-bearing zones as smart zones — no emitter change. Wiring lives in `converter.Run` for `brocade2mds`, gated by the flag. ADR-0011 records the decision.

**Tech Stack:** Go 1.26, `spf13/cobra` CLI, `stretchr/testify` for assertions, existing internal packages (`consolidator`, `converter`, `parser/brocade`, `emitter/mds`, `ir`).

---

## Background the engineer needs

1. **What's already there:**
   - `internal/consolidator/consolidator.go` (B2): `Consolidate(cfg, strict)` takes flat 2-member alias zones, infers `(init, target)` from **zone-name decomposition** (target = trailing component of zone name, or strict `<init>_<target>`), runs a **frequency veto** (target must appear in ≥ 2 zones and ≥ initiator frequency), groups by target, mutates `cfg.Zones` to replace flat zones with role-bearing zones named `<target>_peerzone`, rewrites `cfg.ZoneConfigs[].ZoneNames` to reference the new names. Returns a `Report` describing what changed and what was skipped.
   - `internal/emitter/mds/emitter.go` (PR #5): walks `cfg.Zones`; any zone whose members carry roles is emitted as `member device-alias X init` / `target` / `both`, AND a `zone smart-zoning enable vsan N` line is emitted once per VSAN that contains a roled zone. The emitter doesn't care whether the roles came from a smart-zoned MDS input or a peer-zone Brocade input — same code path.
   - `internal/parser/brocade/parser.go` (PR #5): parses `zonecreate --peerzone …` and the cfgshow `00:02:…` peer-zone marker, producing role-bearing zones. Flat `zonecreate "name", "members"` produces zones with **no roles** — those are what `--smart-consolidate` operates on.
   - `cmd/mds2brocade.go`: shows the existing flag wiring pattern for `--peer-consolidate`, `--consolidate-report`, `--consolidate-strict`.
   - `cmd/brocade2mds.go`: currently has only `--output`. This is where the three new flags go.
2. **The only conceptual difference vs. B2:** The output zone naming convention. B2 names the merged zone `<target>_peerzone` (Brocade idiom). For MDS smart zones, we use `<target>_smartzone`. That single string needs to be a parameter to `consolidator.Consolidate`.
3. **What does NOT change:** the inference heuristic, the frequency veto, the `Report` shape, the MDS emitter (PR #5 already emits smart-zoning correctly when it sees roles).
4. **The verbal convention from the user:** "Same shape as `--peer-consolidate`, just with smart-zoning attribute instead of `--peerzone`." Multi-init merging is wanted.

---

## File Structure

| File | Change | Responsibility after change |
|---|---|---|
| `internal/consolidator/consolidator.go` | **Modify** | Generalize: accept a `nameSuffix` parameter; rename `PeerZoneSummary`→`ConsolidatedZoneSummary`, `PeerName`→`NewName`, `Report.PeerZones`→`Report.Zones` to drop Brocade-specific naming in identifiers (the IR is direction-agnostic) |
| `internal/consolidator/consolidator_test.go` | **Modify** | Update existing assertions for new identifiers + suffix param; add cases that exercise `suffix="smartzone"` |
| `internal/converter/converter.go` | **Modify** | Add `BrocadeConsolidate bool` Option field; in `Run` for `brocade2mds` call `consolidator.Consolidate(cfg, opts.ConsolidateStrict, "smartzone")` after parse/hygiene; rename references to renamed `Report.Zones`; tweak summary line and `writeConsolidateReport` to be direction-agnostic (use the field names that match the IR's actual output type rather than "peer zones") |
| `internal/converter/converter_test.go` | **Modify** | New e2e test: `brocade2mds --smart-consolidate` on a new fixture, expect merged smart zones and the enable directive; init-only fallback test; without the flag, output unchanged |
| `cmd/brocade2mds.go` | **Modify** | Add `--smart-consolidate`, `--consolidate-report`, `--consolidate-strict` flags; pass into `converter.Options` |
| `cmd/mds2brocade.go` | **Modify** | Pass `"peerzone"` suffix through (Option struct now needs `Direction` to drive suffix selection, OR converter.Run picks the suffix based on `opts.Direction` — see Task 2) |
| `testdata/brocade/flat_zones.cfg` | **Create** | Brocade CLI fixture: ~8 flat single-init/single-target zones (`alicreate` + `zonecreate` "name","mem1;mem2"), one SRDF-style edge case, one 3-member zone, one cfg |
| `docs/adr/0011-brocade-flat-to-mds-smart.md` | **Create** | ADR recording the decision: heuristic, opt-in, naming, mirror of ADR-0009 |
| `docs/USER_GUIDE.md` | **Modify** | New subsection "Consolidating Brocade flat zones into MDS smart zones" under the existing consolidation section; add the three new flags to the brocade2mds reference table |

---

## Task 1: Generalize the consolidator (suffix parameter + neutral identifiers)

**Files:**
- Modify: `internal/consolidator/consolidator.go`
- Modify: `internal/consolidator/consolidator_test.go`

- [ ] **Step 1: Write the failing test for the suffix parameter**

Add this test at the end of `internal/consolidator/consolidator_test.go` (after the existing tests):

```go
func TestConsolidate_HonorsNameSuffix(t *testing.T) {
	cfg := &ir.ZoningConfig{
		Aliases: map[string]*ir.Alias{
			"TGT1": {Name: "TGT1", PWWN: "50:00:00:00:00:00:00:01"},
			"ESX1": {Name: "ESX1", PWWN: "10:00:00:00:00:00:00:01"},
			"ESX2": {Name: "ESX2", PWWN: "10:00:00:00:00:00:00:02"},
		},
		Zones: map[string]*ir.Zone{
			"ESX1_TGT1@vsan0": {Name: "ESX1_TGT1", VSAN: 0, Members: []*ir.ZoneMember{
				{Type: "alias", Value: "ESX1"}, {Type: "alias", Value: "TGT1"},
			}},
			"ESX2_TGT1@vsan0": {Name: "ESX2_TGT1", VSAN: 0, Members: []*ir.ZoneMember{
				{Type: "alias", Value: "ESX2"}, {Type: "alias", Value: "TGT1"},
			}},
		},
	}
	report := consolidator.Consolidate(cfg, false, "smartzone")
	require.Len(t, report.Zones, 1, "exactly one consolidated zone")
	require.Equal(t, "TGT1_smartzone", report.Zones[0].NewName)
	_, ok := cfg.Zones["TGT1_smartzone@vsan0"]
	require.True(t, ok, "consolidated zone present in cfg.Zones under suffix-based key")
}
```

This test will not even compile until we change the signature.

- [ ] **Step 2: Run the test to verify the compile failure**

Run: `rtk go test ./internal/consolidator/ -run TestConsolidate_HonorsNameSuffix`
Expected: build fails with errors like `cannot use 3 args` and `report.Zones undefined`.

- [ ] **Step 3: Update the consolidator types**

In `internal/consolidator/consolidator.go`, replace lines 21–40 (the comment + `PeerZoneSummary` + `Report` definitions) with:

```go
// ConsolidatedZoneSummary describes one merged role-bearing zone created by Consolidate.
// In the mds2brocade direction the Brocade emitter renders it as a `zonecreate --peerzone …` block;
// in the brocade2mds direction the MDS emitter renders it as a smart zone (members with init/target
// roles plus a `zone smart-zoning enable vsan N` directive).
type ConsolidatedZoneSummary struct {
	Target      string // the storage-side alias name (the -principal / target-role member)
	NewName     string // the new merged zone's name (Target + "_" + suffix)
	VSAN        int
	Members     []string // initiator alias names, in first-seen order
	SourceZones []string // names of the flat zones that were collapsed into this zone, sorted
}

// SkippedZone records a 2-member zone Consolidate considered but left flat, and why.
type SkippedZone struct {
	Name   string
	Reason string
}

// Report is the result of a Consolidate call: what was created and what was skipped.
type Report struct {
	Zones   []ConsolidatedZoneSummary // sorted by Target
	Skipped []SkippedZone             // sorted by Name
}
```

- [ ] **Step 4: Update `Consolidate` signature and body**

In the same file, change the function signature on line 58 and the doc-comment above it:

```go
// Consolidate collapses flat single-initiator/single-target zones in cfg into
// per-target merged role-bearing zones, mutating cfg in place, and returns a
// Report. The merged zone is named gk.target + "_" + nameSuffix (e.g.
// "TGT1_peerzone" for the Brocade direction, "TGT1_smartzone" for the MDS
// direction); the IR contents are direction-agnostic (target role on the
// principal member, init role on each initiator), and the respective emitters
// (Brocade peer zone, MDS smart zone) render them accordingly.
//
// When strict is false (the default), the target is identified as the member
// alias that is a trailing component of the zone name; when strict is true, the
// zone name must be exactly <init>_<target> or <target>_<init>. Zones it cannot
// confidently classify, and zones that aren't 2-member alias-membered roleless
// zones, are left untouched (the 2-member ones are recorded in Report.Skipped
// with a reason).
func Consolidate(cfg *ir.ZoningConfig, strict bool, nameSuffix string) Report {
```

Inside the function body, replace line 210 (currently `peerName := gk.target + "_peerzone"`) with:

```go
		newName := gk.target + "_" + nameSuffix
```

Then, in the same block (lines ~213–248), substitute `newName` for `peerName` and `Zones` for `PeerZones` in the rest of that block. Specifically:

```go
		// Build the merged role-bearing zone.
		pz := &ir.Zone{
			Name: newName,
			VSAN: gk.vsan,
			Members: []*ir.ZoneMember{
				{Type: "alias", Value: gk.target, Role: "target"},
			},
		}
		for _, init := range g.inits {
			pz.Members = append(pz.Members, &ir.ZoneMember{
				Type:  "alias",
				Value: init,
				Role:  "init",
			})
		}

		// Delete source zones from cfg.
		for _, k := range g.keys {
			delete(cfg.Zones, k)
		}

		// Add merged zone to cfg.
		newKey := fmt.Sprintf("%s@vsan%d", newName, gk.vsan)
		cfg.Zones[newKey] = pz

		// Record the mapping for ZoneConfigs rewrite.
		for _, srcName := range g.sourceZones {
			consolidatedNameToPeer[srcName] = newName
		}

		zones = append(zones, ConsolidatedZoneSummary{
			Target:      gk.target,
			NewName:     newName,
			VSAN:        gk.vsan,
			Members:     g.inits,
			SourceZones: g.sourceZones,
		})
```

And earlier in the function (just before the group loop, ~line 190):

```go
	var zones []ConsolidatedZoneSummary
```

(rename `var peerZones []PeerZoneSummary`).

Update the final sort + return (lines 269–279):

```go
	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Target < zones[j].Target
	})
	sort.Slice(skipped, func(i, j int) bool {
		return skipped[i].Name < skipped[j].Name
	})

	return Report{
		Zones:   zones,
		Skipped: skipped,
	}
```

- [ ] **Step 5: Update existing consolidator tests to the new identifiers**

In `internal/consolidator/consolidator_test.go`, any occurrence of:
- `report.PeerZones` → `report.Zones`
- `.PeerName` → `.NewName`
- `consolidator.Consolidate(cfg, …)` with 2 args → add `"peerzone"` as the third arg (preserves the old behavior all the existing tests assert)
- `PeerZoneSummary` type literal → `ConsolidatedZoneSummary`

Run `rtk grep -n "PeerZones\|PeerName\|PeerZoneSummary\|Consolidate(cfg" internal/consolidator/consolidator_test.go` to find every site, then do the substitutions.

- [ ] **Step 6: Run the full consolidator test suite to verify it passes**

Run: `rtk go test ./internal/consolidator/ -count=1`
Expected: PASS for all subtests including the new `TestConsolidate_HonorsNameSuffix`. If any fails, the substitutions in Step 5 are incomplete — re-run the grep.

- [ ] **Step 7: Commit**

```bash
rtk git add internal/consolidator/consolidator.go internal/consolidator/consolidator_test.go
rtk proxy git commit -m "refactor(consolidator): generalize naming suffix + direction-neutral types

Adds a nameSuffix parameter to Consolidate so the merged zone name can be
'<target>_peerzone' (Brocade direction) or '<target>_smartzone' (MDS
direction). Renames PeerZoneSummary -> ConsolidatedZoneSummary,
PeerName -> NewName, Report.PeerZones -> Report.Zones to drop the
Brocade-specific identifier from a now-bidirectional API; the IR
contents are direction-agnostic (target/init roles on members) and the
respective emitters (Brocade peer zone, MDS smart zone) render them.

Existing B2 callers will pass 'peerzone' in the next commit to preserve
behavior; the new --smart-consolidate path will pass 'smartzone'.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: Wire consolidation into the brocade2mds converter path

**Files:**
- Modify: `internal/converter/converter.go`
- Modify: `internal/converter/converter_test.go`
- Create: `testdata/brocade/flat_zones.cfg`

- [ ] **Step 1: Create the Brocade fixture for the e2e test**

Create `testdata/brocade/flat_zones.cfg` with this exact content:

```
alicreate "TGT1", "50:00:00:00:00:00:00:01"
alicreate "TGT2", "50:00:00:00:00:00:00:02"
alicreate "ESX1", "10:00:00:00:00:00:00:01"
alicreate "ESX2", "10:00:00:00:00:00:00:02"
alicreate "ESX3", "10:00:00:00:00:00:00:03"
alicreate "DR_REPL_A", "50:00:00:00:00:00:00:0a"
alicreate "DR_REPL_B", "50:00:00:00:00:00:00:0b"
zonecreate "ESX1_TGT1", "ESX1;TGT1"
zonecreate "ESX2_TGT1", "ESX2;TGT1"
zonecreate "ESX3_TGT1", "ESX3;TGT1"
zonecreate "ESX1_TGT2", "ESX1;TGT2"
zonecreate "ESX2_TGT2", "ESX2;TGT2"
zonecreate "SRDF_DR_REPL_A_DR_REPL_B", "DR_REPL_A;DR_REPL_B"
zonecreate "ThreeMemberZone", "ESX1;ESX2;TGT1"
cfgcreate "Prod", "ESX1_TGT1;ESX2_TGT1;ESX3_TGT1;ESX1_TGT2;ESX2_TGT2;SRDF_DR_REPL_A_DR_REPL_B;ThreeMemberZone"
cfgenable "Prod"
```

Rationale: TGT1 and TGT2 each have 2+ initiators → consolidatable. The `DR_REPL_*` zone has both members appearing in only 1 zone each → flat (frequency veto). The 3-member zone is excluded by `isCandidate`.

- [ ] **Step 2: Write the failing e2e test for `--smart-consolidate`**

Append to `internal/converter/converter_test.go` (after the existing peer-consolidate e2e tests):

```go
func TestRun_Brocade2MDS_SmartConsolidate(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := converter.Run(converter.Options{
		InputFile:          "../../testdata/brocade/flat_zones.cfg",
		Direction:          "brocade2mds",
		BrocadeConsolidate: true,
	}, &stdout, &stderr)
	require.NoError(t, err)

	out := stdout.String()
	// Smart-zoning must be enabled on the VSAN that holds the merged zones.
	require.Contains(t, out, "zone smart-zoning enable vsan 1",
		"missing smart-zoning enable directive")

	// TGT1 had 3 initiators (ESX1/ESX2/ESX3) → one merged zone named TGT1_smartzone.
	require.Contains(t, out, "zone name TGT1_smartzone vsan 1")
	require.Contains(t, out, "  member device-alias TGT1 target")
	require.Contains(t, out, "  member device-alias ESX1 init")
	require.Contains(t, out, "  member device-alias ESX2 init")
	require.Contains(t, out, "  member device-alias ESX3 init")

	// TGT2 had 2 initiators (ESX1/ESX2) → one merged zone named TGT2_smartzone.
	require.Contains(t, out, "zone name TGT2_smartzone vsan 1")
	require.Contains(t, out, "  member device-alias TGT2 target")

	// The SRDF replication zone (DR_REPL_A / DR_REPL_B both freq=1) stays flat.
	require.Contains(t, out, "zone name SRDF_DR_REPL_A_DR_REPL_B vsan 1",
		"single-occurrence zone should be left flat")
	require.NotContains(t, out, "DR_REPL_A_smartzone")
	require.NotContains(t, out, "DR_REPL_B_smartzone")

	// The 3-member zone is not consolidatable; it stays flat (and roleless).
	require.Contains(t, out, "zone name ThreeMemberZone vsan 1")

	// Original flat zones that WERE consolidated must be gone.
	require.NotContains(t, out, "zone name ESX1_TGT1 vsan")
	require.NotContains(t, out, "zone name ESX2_TGT2 vsan")

	// The zoneset must reference the merged zones, not the originals.
	require.Contains(t, out, "  member TGT1_smartzone")
	require.Contains(t, out, "  member TGT2_smartzone")
	require.NotContains(t, out, "  member ESX1_TGT1")

	// Summary line on stderr.
	require.Contains(t, stderr.String(),
		"Consolidated 5 flat zones into 2 merged zones; ")
}

func TestRun_Brocade2MDS_NoSmartConsolidate_IsUnchanged(t *testing.T) {
	var stdoutWith, stdoutWithout bytes.Buffer
	require.NoError(t, converter.Run(converter.Options{
		InputFile: "../../testdata/brocade/flat_zones.cfg",
		Direction: "brocade2mds",
	}, &stdoutWithout, io.Discard))

	require.NoError(t, converter.Run(converter.Options{
		InputFile:          "../../testdata/brocade/flat_zones.cfg",
		Direction:          "brocade2mds",
		BrocadeConsolidate: false, // explicit
	}, &stdoutWith, io.Discard))

	require.Equal(t, stdoutWithout.String(), stdoutWith.String(),
		"--smart-consolidate=false must produce byte-identical output")
	// No smart-zoning enable when no zones are roled.
	require.NotContains(t, stdoutWithout.String(), "smart-zoning enable")
}
```

If `bytes` or `io` are not yet imported in this test file, add them to the import block at the top of the file.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `rtk go test ./internal/converter/ -run "TestRun_Brocade2MDS_SmartConsolidate|TestRun_Brocade2MDS_NoSmartConsolidate_IsUnchanged" -v`
Expected: build error — `BrocadeConsolidate` is not a field of `converter.Options`.

- [ ] **Step 4: Add `BrocadeConsolidate` to `Options` and wire it through**

In `internal/converter/converter.go`, add a field to the `Options` struct (after the existing `ConsolidateStrict` field around line 34):

```go
	// BrocadeConsolidate, when true (brocade2mds only), collapses flat
	// single-initiator/single-target Brocade zones into per-target MDS smart
	// zones, mirroring --peer-consolidate in the other direction. Shares
	// ConsolidateReport and ConsolidateStrict with the Brocade-direction flag.
	BrocadeConsolidate bool
```

Update the existing Consolidate-call block at lines 78–91 (the `mds2brocade` consolidation) to pass `"peerzone"` as the new third argument and to use the renamed `report.Zones`:

```go
	// Step 2c: Optionally consolidate flat zones into per-target peer zones (mds2brocade).
	if opts.Direction == "mds2brocade" && opts.Consolidate {
		report := consolidator.Consolidate(cfg, opts.ConsolidateStrict, "peerzone")
		consolidated := 0
		for _, pz := range report.Zones {
			consolidated += len(pz.SourceZones)
		}
		fmt.Fprintf(stderr, "Consolidated %d flat zones into %d peer zones; %d zone(s) left flat\n",
			consolidated, len(report.Zones), len(report.Skipped))
		if opts.ConsolidateReport != "" {
			if err := writeConsolidateReport(opts.ConsolidateReport, report, "peer"); err != nil {
				return fmt.Errorf("write consolidate report %q: %w", opts.ConsolidateReport, err)
			}
		}
	}
```

Add a new symmetric block immediately after the mds2brocade block (before the existing Step 3 sanitize block on line 93):

```go
	// Step 2d: Optionally consolidate flat Brocade zones into per-target MDS smart zones (brocade2mds).
	if opts.Direction == "brocade2mds" && opts.BrocadeConsolidate {
		report := consolidator.Consolidate(cfg, opts.ConsolidateStrict, "smartzone")
		consolidated := 0
		for _, pz := range report.Zones {
			consolidated += len(pz.SourceZones)
		}
		fmt.Fprintf(stderr, "Consolidated %d flat zones into %d merged zones; %d zone(s) left flat\n",
			consolidated, len(report.Zones), len(report.Skipped))
		if opts.ConsolidateReport != "" {
			if err := writeConsolidateReport(opts.ConsolidateReport, report, "smart"); err != nil {
				return fmt.Errorf("write consolidate report %q: %w", opts.ConsolidateReport, err)
			}
		}
	}
```

- [ ] **Step 5: Update `writeConsolidateReport` to take a direction-aware label**

Replace the existing `writeConsolidateReport` (currently lines 149–183) entirely:

```go
// writeConsolidateReport writes a human-readable consolidation report to path.
// kind is "peer" (Brocade output) or "smart" (MDS output); it only affects
// the section heading and the rendered zone type word.
func writeConsolidateReport(path string, report consolidator.Report, kind string) error {
	f, err := os.Create(path) //nolint:gosec // path is an operator-supplied CLI argument
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck

	zoneWord := "peer zone"
	headingWord := "Peer zones"
	if kind == "smart" {
		zoneWord = "smart zone"
		headingWord = "Smart zones"
	}

	fmt.Fprintf(f, "# %s consolidation report\n\n", headingWord)
	fmt.Fprintln(f, "# Each "+zoneWord+" below is ONE storage port (the target / -principal member) plus")
	fmt.Fprintln(f, "# the hosts zoned to it — two storage ports / arrays are never combined into one")
	fmt.Fprintln(f, "# "+zoneWord+". This turns single-initiator/single-target zoning into")
	fmt.Fprintln(f, "# single-target/multi-initiator zoning, which the vendor recommends and which keeps")
	fmt.Fprintln(f, "# hosts isolated from one another. Review before applying.")
	fmt.Fprintln(f)
	fmt.Fprintf(f, "## %s created (%d)\n\n", headingWord, len(report.Zones))
	if len(report.Zones) == 0 {
		fmt.Fprintln(f, "(none)")
	}
	for _, pz := range report.Zones {
		fmt.Fprintf(f, "%s %q (VSAN %d)\n", zoneWord, pz.NewName, pz.VSAN)
		fmt.Fprintf(f, "  target/principal: %s\n", pz.Target)
		fmt.Fprintf(f, "  members:          %s\n", strings.Join(pz.Members, ", "))
		fmt.Fprintf(f, "  collapsed %d flat zone(s): %s\n\n", len(pz.SourceZones), strings.Join(pz.SourceZones, ", "))
	}
	fmt.Fprintf(f, "## Zones left flat (%d)\n\n", len(report.Skipped))
	if len(report.Skipped) == 0 {
		fmt.Fprintln(f, "(none)")
	}
	for _, s := range report.Skipped {
		fmt.Fprintf(f, "%s — %s\n", s.Name, s.Reason)
	}
	return nil
}
```

- [ ] **Step 6: Run the new tests to verify they pass**

Run: `rtk go test ./internal/converter/ -run "TestRun_Brocade2MDS_SmartConsolidate|TestRun_Brocade2MDS_NoSmartConsolidate_IsUnchanged" -v`
Expected: PASS on both.

- [ ] **Step 7: Run the full converter suite to verify no regression**

Run: `rtk go test ./internal/converter/ -count=1`
Expected: all existing tests pass too (the `--peer-consolidate` flow is unchanged).

- [ ] **Step 8: Commit**

```bash
rtk git add internal/converter/converter.go internal/converter/converter_test.go testdata/brocade/flat_zones.cfg
rtk proxy git commit -m "feat(converter): wire --smart-consolidate for brocade2mds

Adds Options.BrocadeConsolidate; when set on brocade2mds, runs
consolidator.Consolidate with the 'smartzone' suffix after parse/hygiene.
The MDS emitter (PR #5) already renders the resulting role-bearing zones
as smart zones with the per-VSAN 'zone smart-zoning enable' directive,
so no emitter change is needed.

writeConsolidateReport now takes a direction kind ('peer'|'smart') to
render the right vocabulary. Existing --peer-consolidate behaviour is
preserved (passes 'peerzone'+'peer').

E2E test uses testdata/brocade/flat_zones.cfg covering 5 consolidatable
zones into 2 merged zones, plus a single-occurrence flat zone and a
3-member zone that stay flat.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: CLI flag wiring

**Files:**
- Modify: `cmd/brocade2mds.go`

- [ ] **Step 1: Add the three flags and wire them through**

Replace the entire content of `cmd/brocade2mds.go` with:

```go
package cmd

import (
	"os"

	"github.com/fjacquet/san-conv/internal/converter"
	"github.com/spf13/cobra"
)

var brocade2mdsCmd = &cobra.Command{
	Use:   "brocade2mds [input-file]",
	Short: "Convert Brocade FOS cfgshow or CLI script to Cisco MDS NX-OS commands",
	Long: `brocade2mds parses a Brocade FOS cfgshow output or CLI script and produces
NX-OS CLI commands (device-alias database, zone, zoneset, zoneset activate).

With --smart-consolidate, flat single-initiator/single-target Brocade zones are
collapsed into per-target MDS smart zones (target as principal, hosts as init
members), and "zone smart-zoning enable vsan N" is emitted automatically. The
target is inferred from the zone name (default: the target alias is a trailing
component of the zone name; --consolidate-strict requires exact <init>_<target>).
ALWAYS review --consolidate-report before applying.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile, _ := cmd.Flags().GetString("output")
		consolidate, _ := cmd.Flags().GetBool("smart-consolidate")
		consolidateReport, _ := cmd.Flags().GetString("consolidate-report")
		consolidateStrict, _ := cmd.Flags().GetBool("consolidate-strict")

		return converter.Run(converter.Options{
			InputFile:          args[0],
			Direction:          "brocade2mds",
			OutputFile:         outputFile,
			BrocadeConsolidate: consolidate,
			ConsolidateReport:  consolidateReport,
			ConsolidateStrict:  consolidateStrict,
		}, os.Stdout, os.Stderr)
	},
}

func init() {
	brocade2mdsCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
	brocade2mdsCmd.Flags().Bool("smart-consolidate", false, "consolidate flat single-initiator/single-target zones into per-target MDS smart zones (inferred — review with --consolidate-report)")
	brocade2mdsCmd.Flags().String("consolidate-report", "", "write the consolidation report to this file")
	brocade2mdsCmd.Flags().Bool("consolidate-strict", false, "with --smart-consolidate: require an exact <host>_<target> zone name (default: also consolidate when the target alias is a trailing component of the zone name)")
}
```

- [ ] **Step 2: Build the binary and run an end-to-end check**

Run: `rtk go build -o /tmp/san-conv ./...`
Expected: builds with no errors.

Run: `/tmp/san-conv brocade2mds --smart-consolidate testdata/brocade/flat_zones.cfg 2>&1 | rtk grep -E "smart-zoning enable|TGT1_smartzone|TGT2_smartzone|Consolidated"`
Expected output (order may vary slightly):

```
zone smart-zoning enable vsan 1
zone name TGT1_smartzone vsan 1
zone name TGT2_smartzone vsan 1
Consolidated 5 flat zones into 2 merged zones; 1 zone(s) left flat
```

- [ ] **Step 3: Run `--help` to verify flag docstrings render**

Run: `/tmp/san-conv brocade2mds --help`
Expected: `--smart-consolidate`, `--consolidate-report`, `--consolidate-strict` all appear in the Flags section with their description text.

- [ ] **Step 4: Run the full test suite**

Run: `rtk go test ./...`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
rtk git add cmd/brocade2mds.go
rtk proxy git commit -m "feat(cli): add --smart-consolidate flag to brocade2mds

Mirror of mds2brocade --peer-consolidate, in the other direction.
Shares --consolidate-report and --consolidate-strict semantics.

Long-help calls out that the target is inferred from the zone name and
that --consolidate-report should be reviewed before applying.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: ADR-0011 and USER_GUIDE

**Files:**
- Create: `docs/adr/0011-brocade-flat-to-mds-smart.md`
- Modify: `docs/USER_GUIDE.md`

- [ ] **Step 1: Write ADR-0011**

Create `docs/adr/0011-brocade-flat-to-mds-smart.md`:

```markdown
# ADR-0011: brocade2mds --smart-consolidate (flat Brocade → merged MDS smart zones)

- **Status:** Accepted
- **Date:** 2026-05-13
- **Depends on:** ADR-0008 (smart-zoning → peer-zoning emitter, B1), ADR-0009 (mds2brocade flat-zone consolidation, B2), ADR-0010 (peer-zone → smart-zone round-trip, B3)
- **Companion plan:** docs/superpowers/plans/2026-05-13-smart-consolidate-brocade-to-mds.md

## Context

Real-world Brocade FOS configs typically carry the same flat single-initiator/single-target zone explosion as MDS configs: hundreds of `zonecreate "ESX_TGT", "ESX;TGT"` entries that, when converted naively to MDS, produce hundreds of equally flat NX-OS zones. The Cisco-recommended modern form on MDS is **smart zoning** — one zone per storage port with `init` and `target` role attributes and a per-VSAN `zone smart-zoning enable` directive — which collapses N flat zones to 1 per target.

ADR-0009 already records the inference heuristic (zone-name decomposition + frequency veto) in the `internal/consolidator` package, used today by `mds2brocade --peer-consolidate`. ADR-0010 added the MDS smart-zoning emitter (PR #5). The two pieces compose: feed the consolidator a flat Brocade-sourced IR, set the merged-zone name suffix to `smartzone`, and the MDS emitter renders the result as smart zoning.

## Decision

Add `brocade2mds --smart-consolidate` as the mirror of `mds2brocade --peer-consolidate`:

- **Inference:** unchanged from ADR-0009 (zone-name trailing-component classifier; `--consolidate-strict` for exact `<init>_<target>` mode; cross-zone frequency veto).
- **Output naming:** merged zone is `<target>_smartzone` (the consolidator gains a `nameSuffix` parameter; the existing B2 path passes `"peerzone"`).
- **Output IR:** role-bearing `ir.Zone` (target role on principal, init role on each initiator) — same shape produced by B2.
- **Emission:** the MDS emitter (ADR-0010) already renders role-bearing zones as `member device-alias X init`/`target` plus a `zone smart-zoning enable vsan N` line per affected VSAN. No emitter change.
- **Verification report:** shared `--consolidate-report <file>` flag with `--smart-consolidate`; section heading reflects "Smart zones" instead of "Peer zones".
- **Default behavior:** off. Without `--smart-consolidate`, flat Brocade zones are emitted as flat MDS zones (unchanged).

## Consequences

**Positive:**
- Symmetry with B2 — same heuristic, same safety stance, same review workflow.
- A Brocade-to-MDS migration can land on the modern smart-zoning model in one step.
- Zero risk to the existing default path: opt-in only.

**Negative / cautions:**
- The inference is heuristic. A misclassified target can break host access; mitigated by the conservative veto and the always-on report.
- Renaming `Report.PeerZones → Report.Zones` and `PeerZoneSummary → ConsolidatedZoneSummary` is a one-time internal-API churn; B2 behavior is preserved by passing `"peerzone"` explicitly.

## Alternatives considered

- **Direction-specific consolidator copies.** Rejected: doubles the test surface for ~99% identical logic.
- **Auto-detect the suffix from `opts.Direction` inside `Consolidate`.** Rejected: makes the package depend on a converter-level concept; the suffix is conceptually a presentation hint and belongs at the call site.
- **Keep the `PeerZoneSummary` names.** Rejected: identifier accuracy matters more than churn — the consolidator is now provably bidirectional.

## Implementation

See companion plan. Key shape: one-arg additive to `Consolidate(cfg, strict, nameSuffix)`, a new `BrocadeConsolidate bool` on `converter.Options`, a new converter pipeline step for brocade2mds, three new CLI flags.
```

- [ ] **Step 2: Update USER_GUIDE.md**

Open `docs/USER_GUIDE.md`. Find the section titled "Consolidating zones into peer zones" (the B2 section). Immediately after that section ends, insert a new sibling section:

```markdown
## Consolidating Brocade flat zones into MDS smart zones

`brocade2mds --smart-consolidate` is the mirror of `mds2brocade --peer-consolidate`. It collapses flat single-initiator/single-target Brocade zones into per-target MDS smart zones (one zone per storage port, target as principal, hosts as init members) and adds `zone smart-zoning enable vsan N` automatically.

**The heuristic is identical to `--peer-consolidate`:**
- By default, the target alias must be a trailing component of the zone name (e.g. `ESX04_HBA0_TGT1` → target is `TGT1`).
- With `--consolidate-strict`, the zone name must be exactly `<init>_<target>` or `<target>_<init>`.
- A frequency veto then requires the inferred target to appear in ≥ 2 candidate zones and ≥ as many as the inferred initiator. Anything ambiguous stays flat.

**Example:**

```bash
san-conv brocade2mds --smart-consolidate \
    --consolidate-report consolidation-review.md \
    fabric-a.cfgshow > fabric-a.nxos.txt
```

Review `consolidation-review.md` carefully before applying. A misclassified target causes a host to lose storage access.

**Output shape:** For two flat zones `ESX1_TGT1` and `ESX2_TGT1` sharing target `TGT1`, the merged output is:

```
zone smart-zoning enable vsan 1
zone name TGT1_smartzone vsan 1
  member device-alias TGT1 target
  member device-alias ESX1 init
  member device-alias ESX2 init
```

Combine with `--smart-consolidate` and the existing peer-zone round-trip (already on by default for any input with `--peerzone` blocks) to migrate a fabric to MDS smart zoning in a single conversion.
```

Then find the flag reference table for `brocade2mds` (search for a section listing brocade2mds flags). Add these three rows in the same style as the existing rows:

```markdown
| `--smart-consolidate` | bool | false | Collapse flat single-initiator/single-target zones into per-target MDS smart zones (inferred — review with `--consolidate-report`). |
| `--consolidate-report` | path | (none) | With `--smart-consolidate`, write the verification report to this file. |
| `--consolidate-strict` | bool | false | With `--smart-consolidate`, require exact `<init>_<target>` zone names instead of the trailing-component rule. |
```

If no such reference table exists for `brocade2mds`, add the rows under a new subsection titled "brocade2mds flags" immediately after the new "Consolidating Brocade flat zones into MDS smart zones" section.

- [ ] **Step 3: Spot-check that the docs render cleanly**

Run: `rtk grep -n "smart-consolidate\|TGT1_smartzone\|ADR-0011" docs/USER_GUIDE.md docs/adr/0011-brocade-flat-to-mds-smart.md`
Expected: each file shows the expected anchor lines (USER_GUIDE shows multiple matches for `smart-consolidate`; the ADR file shows its own ID).

- [ ] **Step 4: Commit**

```bash
rtk git add docs/adr/0011-brocade-flat-to-mds-smart.md docs/USER_GUIDE.md
rtk proxy git commit -m "docs: ADR-0011 + USER_GUIDE for --smart-consolidate

ADR-0011 records the decision: mirror of B2 (ADR-0009), shared
consolidator with a nameSuffix parameter, MDS emitter from PR #5 (B3)
already does the smart-zoning rendering — no emitter change.

USER_GUIDE gains a 'Consolidating Brocade flat zones into MDS smart
zones' subsection and three new flag rows for the brocade2mds reference
table.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: Quality gate + customer-capture regression

**Files:**
- (no source changes — verification only)

- [ ] **Step 1: Run the full check pipeline**

Run: `rtk make check`
Expected: build OK, all tests pass, golangci-lint clean. If `make check` is not defined, run the three steps individually:

```bash
rtk go build ./...
rtk go test ./... -count=1
rtk go tool golangci-lint run ./...
```

All three must be green.

- [ ] **Step 2: Manual regression on each customer capture (skipped if `customers/` is empty)**

Run: `rtk ls customers/ 2>/dev/null`
- If the directory is empty or missing, skip to Step 3.
- Otherwise, for each `customers/*.cfgshow` or `*.cli` file:

```bash
/tmp/san-conv brocade2mds <file> > /tmp/baseline.nxos.txt 2>/tmp/baseline.stderr
/tmp/san-conv brocade2mds --smart-consolidate --consolidate-report /tmp/cons.md <file> > /tmp/smart.nxos.txt 2>/tmp/smart.stderr
rtk grep -c "^zone name " /tmp/baseline.nxos.txt /tmp/smart.nxos.txt
rtk head -5 /tmp/smart.stderr
```

Note the zone-count reduction (e.g. baseline 734 → smart 48) and confirm at least one merged zone shape in `/tmp/smart.nxos.txt`:

```bash
rtk grep -A 4 "_smartzone vsan" /tmp/smart.nxos.txt | rtk head -20
```

Spot-check `/tmp/cons.md` for sane target classifications.

- [ ] **Step 3: Build cross-platform binaries (sanity check, no upload)**

Run: `rtk go tool goreleaser build --snapshot --clean 2>&1 | tail -10`
Expected: builds for darwin/arm64, linux/amd64, windows/amd64 with no errors. Artifacts land in `dist/`.

- [ ] **Step 4: Confirm git status is clean and the branch is ready for PR**

Run: `rtk git status -s`
Expected: only ignored/untracked items (`.claude/`, `.rtk/`, `customers/`, possibly `dist/`). No modified files.

Run: `rtk git log maincd..HEAD --oneline`
Expected: exactly four commits (one per task 1–4); each commit ends with the Claude co-author footer.

- [ ] **Step 5: Push the branch and open the PR**

```bash
rtk git push -u origin feat/smart-consolidate
rtk gh pr create --base maincd --head feat/smart-consolidate \
  --title "feat(brocade2mds): --smart-consolidate (flat Brocade → merged MDS smart zones)" \
  --body "Mirror of \`mds2brocade --peer-consolidate\` (B2 / ADR-0009) in the other direction. Reuses the same target-inference heuristic (zone-name trailing-component + frequency veto) and the same \`--consolidate-strict\` / \`--consolidate-report\` flags. The merged zone is named \`<target>_smartzone\`; the MDS emitter from PR #5 renders it as smart zoning with the per-VSAN enable directive — no emitter change. See ADR-0011.

Tests: 5 flat zones → 2 merged smart zones (one with 3 inits, one with 2); a single-occurrence flat zone stays flat (frequency veto); a 3-member zone stays flat (out of scope); without the flag, output is byte-identical to maincd.

🤖 Generated with [Claude Code](https://claude.com/claude-code)"
```

Capture the PR URL printed and report it back.

---

## Self-Review

**1. Spec coverage:** No formal spec was written (we went straight from brainstorm to plan). Cross-checking against the user-stated requirements:
- `--smart-consolidate` flag (brocade2mds only) ✓ Task 3
- Mirrors `--peer-consolidate` shape ✓ Tasks 1–3
- Merges multiple flat zones per target into one smart zone (target=principal, hosts=init members) ✓ Task 2 e2e test asserts this
- Emits `zone smart-zoning enable vsan N` ✓ Task 2 e2e test asserts this; emitter already handles it
- Same conservative fallback + verification report ✓ Tasks 2 + 3
- ADR + USER_GUIDE ✓ Task 4
- Quality gate ✓ Task 5

**2. Placeholder scan:** No "TBD" / "TODO" / "implement later". Every code step shows the exact code; every command step shows the exact command and expected output.

**3. Type consistency:**
- `Consolidate(cfg, strict, nameSuffix string)` — used identically in Task 1 (definition), Task 1 test, Task 2 (both call sites pass a string literal).
- `ConsolidatedZoneSummary` / `NewName` / `Report.Zones` — used in Task 1 (definition), Task 1 test, Task 2 (`writeConsolidateReport` iteration), and the renamed-callsite block in `mds2brocade`-direction code (`for _, pz := range report.Zones`).
- `Options.BrocadeConsolidate` — defined in Task 2, read in Task 3.
- CLI flag names: `--smart-consolidate`, `--consolidate-report`, `--consolidate-strict` consistent across Task 3 definition, Task 3 binary verification, Task 4 USER_GUIDE table.
- `writeConsolidateReport(path, report, kind)` — defined in Task 2 Step 5; both call sites in Task 2 Step 4 pass `"peer"` / `"smart"` respectively.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-13-smart-consolidate-brocade-to-mds.md`. Two execution options:

**1. Subagent-Driven (recommended)** — I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** — Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
