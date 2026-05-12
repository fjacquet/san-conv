# MDS Smart Zoning → Brocade Peer Zoning (Group B1) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert an MDS zone that carries smart-zoning roles (`init`/`target`/`both`) into a Brocade *peer zone* (`zonecreate --peerzone "name" -principal "…" -members "…"`) instead of flattening it and warning "no FOS equivalent".

**Architecture:** Add a `Role` field to `ir.ZoneMember`; the MDS parser captures the role on `member pwwn`/`member device-alias`/`member fcalias` lines and stops emitting the "stripped — no FOS equivalent" warning; the Brocade emitter renders any zone with ≥1 roled member as a peer zone, partitioning `target`/`both`/unroled members into `-principal` and `init` members into `-members` (with informational warnings for `both`/unroled, and a plain-zone fallback when no principals survive). `validator` (the sanitizer) and `converter.Run` are untouched — peer-zone output is unconditional (FOS ≥ 7.4; not gated on `--fos-version`). No new CLI flag.

**Tech Stack:** Go 1.25 (stdlib `regexp`, `strings`, `fmt`), `github.com/stretchr/testify/require` for tests.

**Spec:** `docs/superpowers/specs/2026-05-12-mds-smart-zoning-to-peer-zoning-design.md`
**ADR:** `docs/adr/0008-mds-smart-zoning-to-peer-zoning.md` (already committed in `7e9e71e`)

**Branch:** Work on `feat/peer-zoning-b1`, which is branched from `feat/real-world-config-robustness` (Group A's PR #1 branch) — B1 touches the same parser/emitter/IR files and #1 isn't merged yet. Commit on this branch. Conventional-commit prefixes; every commit message ends with `Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>`. Run commands from the repo root.

---

## Background the engineer needs

- **Cisco smart zoning:** a zone member line can end with a role: `member pwwn 50:06:0e:80:04:7c:00:01 target`, `member device-alias Host-A init`, `member fcalias Array-Port both`. Roles: `init` (initiator), `target`, `both`. In the zone, initiators reach targets and `both` devices; initiators do not reach other initiators.
- **Brocade peer zone (the equivalent):** `zonecreate --peerzone "zonename" -principal "pwwn[;pwwn...]" [-members "pwwn[;pwwn...]"]` — **no comma** after the zone name (unlike the plain `zonecreate "name", "members"` form), semicolon-separated lists with no spaces, alias names accepted just like pWWNs. Principal members reach everyone in the zone; `-members` (non-principal) reach only principals. Verified against Broadcom's Fabric OS Command Reference (`zoneCreate`). *(This is a regular peer zone — not a Target-Driven Peer Zone, which can't be created via CLI; we never touch TDPZ.)*
- **Mapping (from the spec / ADR-0008):** `target` → principal; `both` → principal (a `both` device must reach both the targets and the initiators, which only the principal slot gives it — slightly over-permissive but never under-connects) + a warning; an *unroled* member inside an otherwise-smart-zoned zone → principal (the static config doesn't carry the FCNS-derived role) + a warning; `init` → non-principal. A zone with **no** roled member is emitted exactly as today (plain `zonecreate`).
- **Current code:** `internal/parser/mds/parser.go` has `reMemberPWWN` (captures the WWN) and a separate `reMemberPWWNRole = ^\s+member\s+pwwn\s+\S+\s+(init|target|both)\s*$`; `processZoneMember` matches device-alias / fcalias / pwwn and, for pwwn only, checks `reMemberPWWNRole` and appends `WARN: zone "Z": smart-zoning role "target" on member X stripped — no FOS equivalent`. `internal/emitter/brocade/emitter.go`'s Zones loop builds `members []string` from `m.Value` (skipping `Type == "unsupported"`), skips the zone if empty (with a "no valid FOS members" warning), else emits `zonecreate "name", "m;m"` and records `emittedZones[name] = true` (used to filter `cfgcreate`). `validator.Sanitize` runs only for `mds2brocade` and only does name sanitization. The default `--fos-version` is `8.1+`.
- **Existing fixtures/tests:** `testdata/mds/smart_zoning.cfg` has one zone `SmartZone vsan 10` with 3 `member pwwn … init|target|both` lines + a zoneset. `internal/parser/mds/parser_test.go`'s table case `"smart zoning keywords stripped with warning"` (fixture `smart_zoning.cfg`) currently asserts `cfg.Zones` len 1, `cfg.Warnings` len 3, all members `Type == "pwwn"`, every warning contains `"smart-zoning role"`. `internal/emitter/brocade/emitter_test.go` has a `makeMember(typ, val)` helper (`return &ir.ZoneMember{Type: typ, Value: val}`), `makeZone(name, members...)`, `makeAlias`, `makeZoneConfig`, and table-driven tests asserting on the rendered output string.

---

## File Structure

| File | Status | Responsibility |
|---|---|---|
| `internal/ir/zoningconfig.go` | **modify** | Add `Role string` to `ZoneMember` (`""` \| `"init"` \| `"target"` \| `"both"`). |
| `internal/parser/mds/parser.go` | **modify** | Replace `reMemberPWWNRole` with a type-agnostic `reMemberRole`; in `processZoneMember`, set `ZoneMember.Role` on device-alias / fcalias / pwwn members; remove the "stripped — no FOS equivalent" warning. |
| `internal/parser/mds/parser_test.go` | **modify** | Rewrite the `smart_zoning.cfg` table case (now 0 warnings, roles populated); the extended fixture also covers alias-member roles and an unroled member. |
| `testdata/mds/smart_zoning.cfg` | **modify** | Add a second zone exercising `member device-alias … target`, `member fcalias … init`, and a roleless `member pwwn …` in a smart-zoned zone. |
| `internal/emitter/brocade/emitter.go` | **modify** | In the Zones loop: a zone with ≥1 roled member → peer-zone rendering (partition, `both`/unroled warnings, empty-principal → plain-zone fallback); else unchanged. |
| `internal/emitter/brocade/emitter_test.go` | **modify** | Add cases: peer-zone output; `both`/unroled warnings; no `-members` clause when no inits; init-only → plain-zone fallback + warning; plain zone unchanged (regression). |
| `internal/converter/converter_test.go` | **modify** | End-to-end `mds2brocade` on the extended `smart_zoning.cfg` → stdout has the `--peerzone` line, stderr no longer has "no FOS equivalent". |
| `testdata/mds/smart_zoning_initonly.cfg` | **create** | A smart-zoned zone whose only members are `init` — exercises the plain-zone fallback end-to-end. |
| `internal/converter/converter_test.go` | (same file) | …and a test using `smart_zoning_initonly.cfg`. |
| `docs/USER_GUIDE.md` | **modify** | Short subsection: smart-zoned MDS zones become Brocade peer zones (`zonecreate --peerzone`), require FOS ≥ 7.4, `both`/unroled members are placed as principals (with a warning). |

---

## Task 1: `ir.ZoneMember.Role` + MDS parser captures smart-zoning roles

**Files:**
- Modify: `internal/ir/zoningconfig.go` (`ZoneMember` struct)
- Modify: `internal/parser/mds/parser.go` (member regexes; `processZoneMember`)
- Modify: `testdata/mds/smart_zoning.cfg`
- Modify: `internal/parser/mds/parser_test.go` (the `smart_zoning.cfg` table case)

- [ ] **Step 1: Extend the fixture**

Replace the contents of `testdata/mds/smart_zoning.cfg` with:

```
zone name SmartZone vsan 10
  member pwwn 50:05:0c:00:00:c8:aa:50 init
  member pwwn 50:06:0e:80:04:7c:00:01 target
  member pwwn 50:06:0e:80:04:7c:00:02 both

zone name SmartAliases vsan 10
  member device-alias Host-A init
  member fcalias Array-Port target
  member pwwn 50:05:0c:00:00:c8:aa:99

zoneset name SAN-VSAN10 vsan 10
  member SmartZone
  member SmartAliases
```

- [ ] **Step 2: Update the failing parser test**

In `internal/parser/mds/parser_test.go`, replace the existing table case whose `fixture` is `"smart_zoning.cfg"` (the one named `"smart zoning keywords stripped with warning"`) with:

```go
		{
			name:    "smart zoning roles captured on pwwn and alias members, no warnings",
			fixture: "smart_zoning.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Zones, 2, "want 2 zones (SmartZone, SmartAliases)")
				require.Empty(t, cfg.Warnings, "roles are now captured, not stripped — want 0 warnings")

				sz, ok := cfg.Zones["SmartZone@vsan10"]
				require.True(t, ok, "zone key 'SmartZone@vsan10' must exist")
				require.Len(t, sz.Members, 3)
				for i, m := range sz.Members {
					require.Equal(t, "pwwn", m.Type, "SmartZone member[%d] type", i)
				}
				require.Equal(t, "init", sz.Members[0].Role)
				require.Equal(t, "target", sz.Members[1].Role)
				require.Equal(t, "both", sz.Members[2].Role)

				sa, ok := cfg.Zones["SmartAliases@vsan10"]
				require.True(t, ok, "zone key 'SmartAliases@vsan10' must exist")
				require.Len(t, sa.Members, 3)
				require.Equal(t, "alias", sa.Members[0].Type)
				require.Equal(t, "Host-A", sa.Members[0].Value)
				require.Equal(t, "init", sa.Members[0].Role)
				require.Equal(t, "alias", sa.Members[1].Type)
				require.Equal(t, "Array-Port", sa.Members[1].Value)
				require.Equal(t, "target", sa.Members[1].Role)
				require.Equal(t, "pwwn", sa.Members[2].Type)
				require.Equal(t, "", sa.Members[2].Role, "the roleless member must have an empty Role")
			},
		},
```

- [ ] **Step 3: Run the test to verify it fails**

Run: `go test ./internal/parser/mds/... -run TestParse -v`
Expected: FAIL — `cfg.Zones` is len 1 not 2 (fixture changed but parser/test mismatch), or `m.Role` undefined (compile error: `ZoneMember` has no `Role`).

- [ ] **Step 4: Add the `Role` field to `ir.ZoneMember`**

In `internal/ir/zoningconfig.go`, change the `ZoneMember` struct to:

```go
// ZoneMember represents a single member within a zone.
// Members can be raw pWWNs, alias references, or unsupported types.
type ZoneMember struct {
	// Type indicates the member variant:
	//   "pwwn"        — raw pWWN (always resolvable to FOS)
	//   "alias"       — reference to an Alias by name (device-alias or fcalias)
	//   "unsupported" — interface, fcid, ip-address, etc. (skipped with warning)
	Type string

	// Value holds the member value appropriate to Type:
	//   "pwwn":        the pWWN string
	//   "alias":       the alias name
	//   "unsupported": original member string (for warning message)
	Value string

	// Role is the Cisco smart-zoning role on this member: "" (none), "init",
	// "target", or "both". Brocade emission maps target/both/"" → peer-zone
	// principal and init → non-principal. Always "" for non-MDS sources and for
	// plain (non-smart-zoned) MDS zones.
	Role string
}
```

- [ ] **Step 5: Update the MDS parser**

In `internal/parser/mds/parser.go`:

(a) In the `var (...)` regex block, replace the line

```go
	reMemberPWWNRole      = regexp.MustCompile(`^\s+member\s+pwwn\s+\S+\s+(init|target|both)\s*$`)
```

with

```go
	reMemberRole          = regexp.MustCompile(`^\s+member\s+\S+\s+\S+\s+(init|target|both)\s*$`)
```

(b) Add this helper function next to `processZoneMember` (e.g. immediately above it):

```go
// memberRole extracts a Cisco smart-zoning role ("init"|"target"|"both") from a
// zone member line, or "" if the line has none. It works for pwwn, device-alias,
// and fcalias member lines (the line shape is "  member <type> <name> <role>").
func memberRole(line string) string {
	if m := reMemberRole.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}
```

(c) In `processZoneMember`, replace the three "append member" branches (device-alias, fcalias, pwwn) — currently:

```go
	// device-alias member
	if m := reMemberDeviceAlias.FindStringSubmatch(line); m != nil {
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "alias", Value: m[1]})
		return
	}

	// fcalias member
	if m := reMemberFcAlias.FindStringSubmatch(line); m != nil {
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "alias", Value: m[1]})
		return
	}

	// pWWN member — check for smart-zoning role suffix AFTER extracting WWN
	if m := reMemberPWWN.FindStringSubmatch(line); m != nil {
		wwn := normalizeWWN(m[1])
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "pwwn", Value: wwn})

		// Check for smart-zoning role keyword
		if roleMatch := reMemberPWWNRole.FindStringSubmatch(line); roleMatch != nil {
			role := roleMatch[1]
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q: smart-zoning role %q on member %s stripped — no FOS equivalent",
				zone.Name, role, wwn,
			))
		}
		return
	}
```

with:

```go
	// device-alias member (optionally carrying a smart-zoning role)
	if m := reMemberDeviceAlias.FindStringSubmatch(line); m != nil {
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "alias", Value: m[1], Role: memberRole(line)})
		return
	}

	// fcalias member (optionally carrying a smart-zoning role)
	if m := reMemberFcAlias.FindStringSubmatch(line); m != nil {
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "alias", Value: m[1], Role: memberRole(line)})
		return
	}

	// pWWN member (optionally carrying a smart-zoning role)
	if m := reMemberPWWN.FindStringSubmatch(line); m != nil {
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "pwwn", Value: normalizeWWN(m[1]), Role: memberRole(line)})
		return
	}
```

(d) Check that `fmt` is still used in `parser.go` after removing that warning — it is (many `fmt.Sprintf`/`fmt.Sscanf` calls remain), so no import change.

- [ ] **Step 6: Run the parser tests**

Run: `go test ./internal/parser/mds/... -v`
Expected: `ok` — the updated `smart_zoning.cfg` case passes, all other parser tests still pass.

- [ ] **Step 7: Run the full suite + format + vet**

Run: `gofmt -l internal/ir/ internal/parser/mds/ && go vet ./... && go test ./...`
Expected: no `gofmt` output, no `go vet` output, all packages `ok`. *(The Brocade emitter still produces a plain `zonecreate` for the roled zone at this point — that's expected; Task 2 fixes the emitter. No emitter test asserts on `smart_zoning.cfg` output, so nothing breaks here.)*

- [ ] **Step 8: Commit**

```bash
git add internal/ir/zoningconfig.go internal/parser/mds/parser.go internal/parser/mds/parser_test.go testdata/mds/smart_zoning.cfg
git commit -m "$(cat <<'EOF'
feat(parser/mds): capture smart-zoning roles instead of stripping them

ir.ZoneMember gains a Role field ("" | "init" | "target" | "both"). The
MDS parser records the role on member pwwn / device-alias / fcalias
lines and no longer emits the "smart-zoning role … no FOS equivalent"
warning — peer-zone emission (next commit) is the FOS equivalent.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Brocade emitter — render peer zones for roled zones

**Files:**
- Modify: `internal/emitter/brocade/emitter.go` (the Zones loop)
- Modify: `internal/emitter/brocade/emitter_test.go` (new cases)

- [ ] **Step 1: Write the failing emitter tests**

In `internal/emitter/brocade/emitter_test.go`, add this test function at the end of the file:

```go
func TestEmit_PeerZones(t *testing.T) {
	t.Parallel()

	roled := func(typ, val, role string) *ir.ZoneMember {
		return &ir.ZoneMember{Type: typ, Value: val, Role: role}
	}

	t.Run("smart-zoned zone emits zonecreate --peerzone with partitioned members", func(t *testing.T) {
		t.Parallel()
		cfg := &ir.ZoningConfig{
			SourceFormat: "mds-nxos",
			Aliases:      map[string]*ir.Alias{},
			Zones: map[string]*ir.Zone{
				"PZ": {Name: "PZ", VSAN: 0, Members: []*ir.ZoneMember{
					roled("pwwn", "10:00:00:00:c9:00:00:01", "init"),
					roled("pwwn", "50:06:0e:80:00:00:00:01", "target"),
				}},
			},
			ZoneConfigs: map[string]*ir.ZoneConfig{},
		}
		var buf bytes.Buffer
		require.NoError(t, Emit(cfg, &buf, false))
		require.Contains(t, buf.String(),
			`zonecreate --peerzone "PZ" -principal "50:06:0e:80:00:00:00:01" -members "10:00:00:00:c9:00:00:01"`)
		require.Empty(t, cfg.Warnings)
	})

	t.Run("both and roleless members go to principal with a warning", func(t *testing.T) {
		t.Parallel()
		cfg := &ir.ZoningConfig{
			SourceFormat: "mds-nxos",
			Aliases:      map[string]*ir.Alias{},
			Zones: map[string]*ir.Zone{
				"PZ": {Name: "PZ", VSAN: 0, Members: []*ir.ZoneMember{
					roled("pwwn", "10:00:00:00:c9:00:00:01", "init"),
					roled("pwwn", "50:06:0e:80:00:00:00:01", "target"),
					roled("pwwn", "50:06:0e:80:00:00:00:02", "both"),
					roled("alias", "Mystery", ""),
				}},
			},
			ZoneConfigs: map[string]*ir.ZoneConfig{},
		}
		var buf bytes.Buffer
		require.NoError(t, Emit(cfg, &buf, false))
		out := buf.String()
		require.Contains(t, out,
			`zonecreate --peerzone "PZ" -principal "50:06:0e:80:00:00:00:01;50:06:0e:80:00:00:00:02;Mystery" -members "10:00:00:00:c9:00:00:01"`)
		joined := strings.Join(cfg.Warnings, "\n")
		require.Contains(t, joined, `zone "PZ": smart-zoning role "both" on member 50:06:0e:80:00:00:00:02 → emitted as a peer-zone principal`)
		require.Contains(t, joined, `zone "PZ": member Mystery has no smart-zoning role → emitted as a peer-zone principal`)
	})

	t.Run("no init members means no -members clause", func(t *testing.T) {
		t.Parallel()
		cfg := &ir.ZoningConfig{
			SourceFormat: "mds-nxos",
			Aliases:      map[string]*ir.Alias{},
			Zones: map[string]*ir.Zone{
				"PZ": {Name: "PZ", VSAN: 0, Members: []*ir.ZoneMember{
					roled("pwwn", "50:06:0e:80:00:00:00:01", "target"),
					roled("pwwn", "10:00:00:00:c9:00:00:01", "both"),
				}},
			},
			ZoneConfigs: map[string]*ir.ZoneConfig{},
		}
		var buf bytes.Buffer
		require.NoError(t, Emit(cfg, &buf, false))
		out := buf.String()
		require.Contains(t, out, `zonecreate --peerzone "PZ" -principal "50:06:0e:80:00:00:00:01;10:00:00:00:c9:00:00:01"`)
		require.NotContains(t, out, "-members")
	})

	t.Run("all-init smart-zoned zone falls back to a plain zone with a warning", func(t *testing.T) {
		t.Parallel()
		cfg := &ir.ZoningConfig{
			SourceFormat: "mds-nxos",
			Aliases:      map[string]*ir.Alias{},
			Zones: map[string]*ir.Zone{
				"PZ": {Name: "PZ", VSAN: 0, Members: []*ir.ZoneMember{
					roled("pwwn", "10:00:00:00:c9:00:00:01", "init"),
					roled("pwwn", "10:00:00:00:c9:00:00:02", "init"),
				}},
			},
			ZoneConfigs: map[string]*ir.ZoneConfig{},
		}
		var buf bytes.Buffer
		require.NoError(t, Emit(cfg, &buf, false))
		out := buf.String()
		require.Contains(t, out, `zonecreate "PZ", "10:00:00:00:c9:00:00:01;10:00:00:00:c9:00:00:02"`)
		require.NotContains(t, out, "--peerzone")
		require.Contains(t, strings.Join(cfg.Warnings, "\n"),
			`zone "PZ": peer zone has no principal members after resolution — emitted as a plain zone`)
	})

	t.Run("zone with no roles is still a plain zonecreate", func(t *testing.T) {
		t.Parallel()
		cfg := &ir.ZoningConfig{
			SourceFormat: "mds-nxos",
			Aliases:      map[string]*ir.Alias{},
			Zones: map[string]*ir.Zone{
				"PlainZ": {Name: "PlainZ", VSAN: 0, Members: []*ir.ZoneMember{
					{Type: "pwwn", Value: "10:00:00:00:c9:00:00:01"},
					{Type: "pwwn", Value: "50:06:0e:80:00:00:00:01"},
				}},
			},
			ZoneConfigs: map[string]*ir.ZoneConfig{},
		}
		var buf bytes.Buffer
		require.NoError(t, Emit(cfg, &buf, false))
		out := buf.String()
		require.Contains(t, out, `zonecreate "PlainZ", "10:00:00:00:c9:00:00:01;50:06:0e:80:00:00:00:01"`)
		require.NotContains(t, out, "--peerzone")
		require.Empty(t, cfg.Warnings)
	})
}
```

If `internal/emitter/brocade/emitter_test.go` does not already import `strings`, add `"strings"` to its import block (the head of the file shows `bytes`, `strings`, `testing`, `ir`, `require` are already imported — so no change needed; verify).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/emitter/brocade/... -run TestEmit_PeerZones -v`
Expected: FAIL — the emitter currently produces `zonecreate "PZ", "…;…"` (plain), not `--peerzone`.

- [ ] **Step 3: Implement peer-zone rendering**

In `internal/emitter/brocade/emitter.go`, replace the body of the `for _, key := range zoneKeys` loop in the Zones section. Currently it is:

```go
		for _, key := range zoneKeys {
			zone := cfg.Zones[key]

			// Build valid member list — skip unsupported members.
			var members []string
			for _, m := range zone.Members {
				if m.Type == "unsupported" {
					continue
				}
				members = append(members, m.Value)
			}

			// Skip zones that have no valid FOS members after filtering.
			if len(members) == 0 {
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
					"zone %q has no valid FOS members after filtering unsupported types — skipped",
					zone.Name,
				))
				continue
			}

			fmt.Fprintf(w, "zonecreate \"%s\", \"%s\"\n", zone.Name, strings.Join(members, ";"))
			emittedZones[zone.Name] = true
		}
```

Replace it with:

```go
		for _, key := range zoneKeys {
			zone := cfg.Zones[key]

			if zoneHasRole(zone) {
				emitPeerZone(zone, cfg, w, emittedZones)
				continue
			}

			// Plain zone — build valid member list, skipping unsupported members.
			var members []string
			for _, m := range zone.Members {
				if m.Type == "unsupported" {
					continue
				}
				members = append(members, m.Value)
			}
			if len(members) == 0 {
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
					"zone %q has no valid FOS members after filtering unsupported types — skipped",
					zone.Name,
				))
				continue
			}
			fmt.Fprintf(w, "zonecreate \"%s\", \"%s\"\n", zone.Name, strings.Join(members, ";"))
			emittedZones[zone.Name] = true
		}
```

Then add these two helper functions to `internal/emitter/brocade/emitter.go` (e.g. immediately after `func Emit(...)` or just before `sortedStringKeys`):

```go
// zoneHasRole reports whether any member of the zone carries a Cisco
// smart-zoning role — in which case the zone is emitted as a Brocade peer zone.
func zoneHasRole(zone *ir.Zone) bool {
	for _, m := range zone.Members {
		if m.Role != "" {
			return true
		}
	}
	return false
}

// emitPeerZone renders a smart-zoned MDS zone as a Brocade peer zone:
//   target/both/(roleless) members → -principal,  init members → -members.
// 'both' and roleless members each get an informational warning. If no principal
// members survive, it falls back to a plain zonecreate (with a warning); if no
// members survive at all, the zone is skipped (with the standard warning).
func emitPeerZone(zone *ir.Zone, cfg *ir.ZoningConfig, w io.Writer, emittedZones map[string]bool) {
	var principal, nonPrincipal []string
	for _, m := range zone.Members {
		if m.Type == "unsupported" {
			continue
		}
		switch m.Role {
		case "init":
			nonPrincipal = append(nonPrincipal, m.Value)
		case "target":
			principal = append(principal, m.Value)
		case "both":
			principal = append(principal, m.Value)
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q: smart-zoning role \"both\" on member %s → emitted as a peer-zone principal",
				zone.Name, m.Value))
		default: // "" — roleless member inside a smart-zoned zone
			principal = append(principal, m.Value)
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q: member %s has no smart-zoning role → emitted as a peer-zone principal (over-permissive); review",
				zone.Name, m.Value))
		}
	}

	if len(principal) == 0 && len(nonPrincipal) == 0 {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q has no valid FOS members after filtering unsupported types — skipped",
			zone.Name))
		return
	}
	if len(principal) == 0 {
		// A peer zone requires at least one principal — fall back to a plain zone.
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q: peer zone has no principal members after resolution — emitted as a plain zone",
			zone.Name))
		fmt.Fprintf(w, "zonecreate \"%s\", \"%s\"\n", zone.Name, strings.Join(nonPrincipal, ";"))
		emittedZones[zone.Name] = true
		return
	}

	line := fmt.Sprintf("zonecreate --peerzone \"%s\" -principal \"%s\"", zone.Name, strings.Join(principal, ";"))
	if len(nonPrincipal) > 0 {
		line += fmt.Sprintf(" -members \"%s\"", strings.Join(nonPrincipal, ";"))
	}
	fmt.Fprintln(w, line)
	emittedZones[zone.Name] = true
}
```

Check `emitter.go`'s imports: it already imports `fmt`, `io`, `sort`, `strings`, and `internal/ir` — `emitPeerZone`'s `io.Writer` parameter needs `io`, which is already imported. No import change.

- [ ] **Step 4: Run the emitter tests**

Run: `go test ./internal/emitter/brocade/... -v`
Expected: `ok` — `TestEmit_PeerZones` (all 5 subtests) passes; all existing emitter tests still pass.

- [ ] **Step 5: Run the full suite + format + vet**

Run: `gofmt -l internal/emitter/brocade/ && go vet ./... && go test ./...`
Expected: no `gofmt` output, no `go vet` output, all packages `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/emitter/brocade/emitter.go internal/emitter/brocade/emitter_test.go
git commit -m "$(cat <<'EOF'
feat(emitter/brocade): emit zonecreate --peerzone for smart-zoned zones

A zone with any roled member becomes a Brocade peer zone:
target/both/roleless → -principal, init → -members. 'both' and roleless
members get an informational warning; a zone with no principal members
falls back to a plain zonecreate. Plain zones are unchanged.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Converter end-to-end test, init-only fixture, USER_GUIDE

**Files:**
- Create: `testdata/mds/smart_zoning_initonly.cfg`
- Modify: `internal/converter/converter_test.go`
- Modify: `docs/USER_GUIDE.md`

- [ ] **Step 1: Create the init-only fixture**

Create `testdata/mds/smart_zoning_initonly.cfg`:

```
zone name InitOnly vsan 10
  member pwwn 10:00:00:00:c9:aa:00:01 init
  member pwwn 10:00:00:00:c9:aa:00:02 init

zoneset name ZS-VSAN10 vsan 10
  member InitOnly

zoneset activate name ZS-VSAN10 vsan 10
```

- [ ] **Step 2: Write the failing converter tests**

In `internal/converter/converter_test.go`, add:

```go
// Smart-zoned MDS zone converts to a Brocade peer zone (no "no FOS equivalent" warning).
func TestRun_MDS2Brocade_PeerZone(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "../../testdata/mds/smart_zoning.cfg",
		Direction: "mds2brocade",
		// default FOSVersion (8.1+); peer-zone output is not version-gated anyway
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	require.Contains(t, out, `zonecreate --peerzone "SmartZone" -principal `)
	require.Contains(t, out, `zonecreate --peerzone "SmartAliases" -principal `)
	require.NotContains(t, stderr.String(), "no FOS equivalent")
}

// A smart-zoned zone with only init members falls back to a plain zone + warning.
func TestRun_MDS2Brocade_PeerZoneInitOnlyFallback(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "../../testdata/mds/smart_zoning_initonly.cfg",
		Direction: "mds2brocade",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), `zonecreate "InitOnly", "10:00:00:00:c9:aa:00:01;10:00:00:00:c9:aa:00:02"`)
	require.NotContains(t, stdout.String(), "--peerzone")
	require.Contains(t, stderr.String(), `zone "InitOnly": peer zone has no principal members after resolution`)
}
```

- [ ] **Step 3: Run the new converter tests**

Run: `go test ./internal/converter/... -run 'TestRun_MDS2Brocade_PeerZone' -v`
Expected: both PASS. (No production-code change needed — Tasks 1 & 2 already implemented this; these tests verify it end-to-end.)

- [ ] **Step 4: Run the full suite**

Run: `go test ./...`
Expected: all `ok`.

- [ ] **Step 5: Update USER_GUIDE.md**

Open `docs/USER_GUIDE.md`, find a sensible spot (near the description of zone conversion output / warnings — e.g. after the section that lists what `mds2brocade` produces, or after the warnings section), and add a subsection. Use this text exactly:

```markdown
### Smart zoning → peer zones

If a Cisco MDS zone uses smart zoning (members tagged `init`, `target`, or
`both`), `mds2brocade` converts it to a Brocade **peer zone**:

```
zonecreate --peerzone "ZoneName" -principal "<targets>" -members "<initiators>"
```

- `target` and `both` members, plus any member with no role tag, become
  `-principal` members; a warning is emitted for each `both`/untagged member
  (placing them as principals is connectivity-safe but slightly over-permissive).
- `init` members become non-principal `-members`.
- If a smart-zoned zone has no principal members (all `init`), it is emitted as a
  plain `zonecreate` instead, with a warning.

Peer-zone output requires **Fabric OS ≥ 7.4** on the target switch (peer zoning
was introduced in FOS 7.4). It is emitted regardless of `--fos-version`.
```

(If `USER_GUIDE.md` uses a different heading level or a table of contents, match the surrounding style — adjust the `###` and add a TOC entry if the doc has one.)

- [ ] **Step 6: Commit**

```bash
git add testdata/mds/smart_zoning_initonly.cfg internal/converter/converter_test.go docs/USER_GUIDE.md
git commit -m "$(cat <<'EOF'
test(converter): cover mds2brocade peer-zone output end-to-end; doc peer zoning

Adds an end-to-end test that a smart-zoned MDS config produces
`zonecreate --peerzone` lines (and no "no FOS equivalent" warning), an
init-only fallback test, and a USER_GUIDE subsection on peer-zone output
and its FOS ≥ 7.4 requirement.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4: Quality gate + customer-capture regression

**Files:** none (verification only)

- [ ] **Step 1: Run the full quality gate**

Run: `make check`
Expected: `fmt` + `vet` + `lint` + `test` all green (Group A made `make check` green; this work must keep it green).

- [ ] **Step 2: Confirm no regression on the four customer captures**

The four captures have no smart zoning, so `mds2brocade` output must be byte-identical to Group A's. Run:

```bash
go build -o san-conv .
for f in F1D1 F1D3 F2D2 F2D4; do
  ./san-conv mds2brocade --fos-version 8.1+ --output /tmp/b1_$f.txt customers/$f.txt 2>/tmp/b1_e_$f.txt
  echo "=== $f ===  WARN=$(grep -c WARN /tmp/b1_e_$f.txt)  zonecreate=$(grep -c zonecreate /tmp/b1_$f.txt)  peerzone=$(grep -c -- '--peerzone' /tmp/b1_$f.txt)  | $(grep '^Summary' /tmp/b1_e_$f.txt)"
done
```

Expected per file: `WARN=1` (the multi-VSAN breakdown, unchanged from Group A), `zonecreate=734` (F1D1/F1D3) / `731` (F2D2/F2D4), `peerzone=0` (no smart zoning in these inputs), Summary line unchanged from Group A's numbers.

- [ ] **Step 3: Smoke-test peer-zone output on a smart-zoned input**

Run: `./san-conv mds2brocade testdata/mds/smart_zoning.cfg 2>&1`
Expected: stdout contains `zonecreate --peerzone "SmartZone" -principal "50:06:0e:80:04:7c:00:01;50:06:0e:80:04:7c:00:02" -members "50:05:0c:00:00:c8:aa:50"` and `zonecreate --peerzone "SmartAliases" -principal "Array-Port;50:05:0c:00:00:c8:aa:99" -members "Host-A"`; stderr contains the `"both"`-member warning and the roleless-member warning; no "no FOS equivalent" warning anywhere.

- [ ] **Step 4: Record results in the PR description**

When opening the B1 PR, note: the four customer captures are byte-identical to Group A (no smart zoning), and paste the `smart_zoning.cfg` peer-zone output from Step 3 as the worked example. (No commit — documentation for the reviewer.)

---

## Self-Review

**Spec coverage:**
- `ir.ZoneMember.Role` → Task 1 Step 4. ✔
- Parser captures roles on pwwn/device-alias/fcalias; removes the "no FOS equivalent" warning → Task 1 Step 5. ✔
- Emitter renders `zonecreate --peerzone … -principal … -members …` for roled zones; `-members` omitted when empty → Task 2 Step 3. ✔
- Mapping `target`→principal, `both`→principal+warn, roleless→principal+warn, `init`→non-principal → Task 2 Step 3 (`emitPeerZone` switch). ✔
- Degenerate fallbacks (no principals → plain zone + warn; no members → skip) → Task 2 Step 3. ✔
- Plain zones unchanged → Task 2 Step 3 (the `else` branch) + a regression test in Task 2 Step 1. ✔
- `validator` / `converter.Run` untouched → no task modifies them. ✔
- ADR-0008 → already committed in `7e9e71e`; referenced in the plan header. ✔
- USER_GUIDE peer-zone subsection → Task 3 Step 5. ✔
- Tests: parser (roles captured, alias-member roles, roleless member, no warnings) → Task 1 Steps 1–2; emitter (peer output, both/roleless warnings, no-`-members`, init-only fallback, plain unchanged) → Task 2 Step 1; converter end-to-end + init-only fallback → Task 3 Step 2. ✔
- Fixtures: `smart_zoning.cfg` extended → Task 1 Step 1; `smart_zoning_initonly.cfg` created → Task 3 Step 1. ✔
- Customer-capture no-regression check → Task 4 Step 2. ✔
- Non-goals (consolidation, brocade2mds `--peerzone` parsing, `--fos-version pre-8.1` deprecation, TDPZ, new CLI flag) → not in any task, as intended. ✔

**Placeholder scan:** No `TBD`/`TODO`/"handle edge cases"/"similar to Task N" — every code step shows complete code, every command shows expected output. The USER_GUIDE step gives exact text plus a "match surrounding style" note (a real instruction, not a placeholder).

**Type consistency:** `ir.ZoneMember{Type, Value, Role}` defined in Task 1 Step 4 and used identically in Tasks 1–3. `reMemberRole` defined and used in Task 1 Step 5. Helpers `memberRole(line string) string` (Task 1), `zoneHasRole(*ir.Zone) bool` and `emitPeerZone(*ir.Zone, *ir.ZoningConfig, io.Writer, map[string]bool)` (Task 2) are referenced only where defined. Warning string literals are reused verbatim between `emitPeerZone` (Task 2 Step 3) and the test assertions (Task 2 Step 1, Task 3 Step 2): `smart-zoning role "both" on member %s → emitted as a peer-zone principal`, `member %s has no smart-zoning role → emitted as a peer-zone principal (over-permissive); review`, `peer zone has no principal members after resolution — emitted as a plain zone`. The `--peerzone` line format (`zonecreate --peerzone "name" -principal "…" -members "…"` — no comma after the name) is consistent across `emitPeerZone`, all emitter tests, the converter tests, and Task 4's smoke test. ✔

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-12-mds-smart-zoning-to-peer-zoning.md`. Two execution options:

1. **Subagent-Driven (recommended)** — fresh subagent per task, two-stage review (spec then code quality) between tasks, continuous execution.
2. **Inline Execution** — execute the tasks in this session in batches with checkpoints.

Which approach?
