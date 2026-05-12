# Design: Flat-zone consolidation into Brocade peer zones (Group B2)

- **Date:** 2026-05-12
- **Status:** Approved — pending implementation
- **Author:** Frederic Jacquet (with Claude)
- **Scope tag:** Group B2. Builds on B1 (smart-zoning → peer-zoning emitter, ADR-0008).
- **Companion ADR:** `docs/adr/0009-peer-zone-consolidation.md` (to be created)

## Background

Real-world MDS configs carry the classic single-initiator/single-target zone explosion: ~730 flat 2-member zones per fabric in the customer captures, regularly named `<initiator>_<target>` (`ESX04_HBA0_GVAMAX11_FA1D04`, `CLU151_HBA0_GVAMAX01_FA2D04`, `ESX212-A0_alletra-swigva01-200-N0P1`, …) where the zone's two members are exactly those two aliases. Brocade best practice for a modern fabric is a *peer zone* per storage port — one principal (target) plus all the initiators zoned to it — collapsing those ~730 zones to ~30–50.

Group B1 (merged) built the emitter half: an `ir.Zone` whose members carry `Role` (`target` → `-principal`, `init` → `-members`) renders as `zonecreate --peerzone "name" -principal "…" -members "…"`. B2 adds the consolidation: an **opt-in** transform that infers which member of each flat zone is the target, groups flat zones by target, and rewrites the IR into per-target peer zones — feeding B1's emitter unchanged. The configs have no smart-zoning roles, so the target/initiator split is *inferred*; the heuristic is conservative (leave a zone flat when not confident) and produces a verification report. B2 also folds in a small set of always-on config-hygiene warnings (dangling alias refs, orphaned zones/aliases, empty/single-member zones).

## Goals

- `mds2brocade --peer-consolidate` collapses flat single-initiator/single-target zones into per-target peer zones, replacing the original flat zones and rebuilding `cfgcreate` accordingly; un-consolidated zones stay flat.
- The target/initiator inference uses two signals: **zone-name decomposition** (primary, deterministic) and **cross-zone member frequency** (secondary, confirmation/veto). A zone is consolidated only when the verdict is unambiguous and not contradicted; otherwise it's left flat and the reason is recorded.
- A verification report: a summary line always on stderr; `--consolidate-report <file>` writes the full per-peer-zone / per-skipped-zone detail.
- Always-on basic config-hygiene warnings: dangling alias references, zones with 0 or 1 members, aliases defined but unused, zones not in any zoneset.
- ADR-0009 records the heuristic and the safety stance. USER_GUIDE documents the flag and the hygiene warnings.
- Tests lock all of it down.

## Non-Goals (YAGNI)

- 3+-member zone consolidation (would fall back to frequency-only classification).
- WWN-OUI vendor lookup or alias-name-prefix heuristics as additional signals.
- `brocade2mds` parsing of `zonecreate --peerzone …` back into MDS smart zoning (carried-over round-trip gap from B1).
- A fuller "config linter" mode (the hygiene checks here are the basic set; a richer one could be Group B3).
- Deprecating `--fos-version pre-8.1`.
- Consolidation in the `brocade2mds` direction (Brocade is already single-fabric; the flag is mds2brocade-only).

## Architecture

```
parse → hygiene.Check (always) → filterVSAN (if --vsan) → consolidator.Consolidate (if --peer-consolidate)
      → validator.Sanitize (mds2brocade) → brocadeemitter.Emit
```

`consolidator.Consolidate` produces role-bearing `ir.Zone`s; B1's `emitPeerZone`/`zoneHasRole` already render them — **no emitter change**. The transform mirrors the existing `filterVSAN` placement in `converter.Run`.

### New: `internal/consolidator`

`func Consolidate(cfg *ir.ZoningConfig) Report` — mutates `cfg`, returns a `Report`. Pure, table-testable.

1. **Frequency map** — per alias name, count the *eligible flat zones* it appears in. Eligible flat zone = exactly 2 members, both `Type=="alias"`, both `Role==""`.
2. **Per-zone classification** — for an eligible flat zone `Z` with member names `A`, `B`:
   - *Name decomposition:* `Z.Name == A+"_"+B` → init=A, target=B; `Z.Name == B+"_"+A` → init=B, target=A; both/neither → no verdict (try a case-fold equality as a fallback first).
   - *Frequency veto:* require `freq(target) >= 2` **and** `freq(target) >= freq(init)`; else not consolidatable.
   - Verdict survives → consolidatable `(init, target)`. Otherwise → leave flat, record reason ("zone name does not decompose to its two members" / "inferred target appears in too few zones" / "inferred target is less frequent than the inferred initiator" / "not a 2-member alias zone").
3. **Group by target** — for each target `T` with consolidatable zones, collect the initiators (dedup, first-seen order). Build `ir.Zone{ Name: T+"_peerzone", VSAN: <common VSAN of the source zones>, Members: [{alias,T,"target"}] ++ [{alias,Ii,"init"} …] }`. (If the source zones disagree on VSAN — unlikely — skip that target and log it.) If a target ends up with only one initiator → leave its zone(s) flat and log it (a 1+1 peer zone has no benefit).
4. **Rewrite the IR** — delete consolidated source zones from `cfg.Zones`; add the peer zones (keyed `<peername>@vsan<N>`, matching the parser's keying). For each `cfg.ZoneConfigs[].ZoneNames`: replace each consolidated source-zone name with its target's peer-zone name, dedup, keep first-appearance order; un-consolidated names unchanged.
5. **Report** — `Report{ PeerZones []PeerZoneSummary{Target, PeerName, VSAN, Members []string, SourceZones []string}; Skipped []SkippedZone{Name, Reason} }`. The caller renders it.

### New: `internal/hygiene`

`func Check(cfg *ir.ZoningConfig)` — appends warnings (ADR-0002 "warn and continue"); no flag; runs for both directions, every run, after parse, before sanitize/consolidate (so it sees the original zones):
- alias-type zone member whose `Value` ∉ `cfg.Aliases` → `WARN: zone "Z": member alias "X" is not defined — dangling reference`
- zone with 0 members → `WARN: zone "Z" has no members`
- zone with exactly 1 member → `WARN: zone "Z" has a single member — nothing to communicate with`
- aliases in `cfg.Aliases` referenced by no zone member → one `WARN: N alias(es) defined but unused in any zone: a, b, c … (and M more)` (truncate at ~10)
- zones in `cfg.Zones` referenced by no `cfg.ZoneConfigs[].ZoneNames` → one `WARN: N zone(s) not in any zoneset: a, b, c … (and M more)` (truncate at ~10)

### Changed: `internal/converter/converter.go`

- `Options` gains `Consolidate bool` and `ConsolidateReport string`.
- In `Run`: `hygiene.Check(cfg)` always (after parse, before `filterVSAN`/sanitize). If `opts.Direction == "mds2brocade" && opts.Consolidate`: `report := consolidator.Consolidate(cfg)` (after `filterVSAN`, before `validator.Sanitize`); print the summary line to stderr (`Consolidated %d flat zones into %d peer zones; %d zone(s) left flat`); if `opts.ConsolidateReport != ""`, `os.Create` it, defer-close, write the full per-peer-zone + per-skipped-zone detail.

### Changed: `cmd/mds2brocade.go`, `cmd/root.go`

- `--peer-consolidate` (bool, default false) — "consolidate flat single-initiator/single-target zones into per-target Brocade peer zones (inferred; see --consolidate-report)".
- `--consolidate-report` (string, default "") — "write the consolidation verification report to this file".
- Read in `RunE`, pass into `converter.Options`. Not added to `brocade2mds`.

### New/changed docs

- `docs/adr/0009-peer-zone-consolidation.md` (Accepted): the heuristic (name decomposition + frequency veto), strictly-2-member scope, opt-in/default-off, `_peerzone` naming, replace-flat-zones behavior, safety stance (conservative; verification report; misclassification → host loses storage), builds on ADR-0008. Notes the basic hygiene checks.
- `docs/USER_GUIDE.md`: "Consolidating zones into peer zones" subsection (what `--peer-consolidate` does, the heuristic in plain terms, "always review the report", FOS ≥ 7.4 carried from B1) + a "Config hygiene warnings" note.

## Error handling

ADR-0002 "warn and continue" throughout: every skip (un-consolidatable zone, VSAN-disagreement, single-initiator peer zone) and every hygiene finding is a `WARN` / a report entry, never fatal. `--consolidate-report` file-create failure → an error (IO error, like `--output`).

## Testing

- `internal/consolidator/consolidator_test.go` — table-driven (build IR directly): clean `init_target` pair → one role-bearing peer zone; many zones for one target → one peer zone, deduped initiators; target in only 1 zone → flat; `_SRDF`-suffixed / unrelated name → flat with the right reason; 3-member / 1-member / pwwn-membered / already-roled zone → flat; `cfg.ZoneConfigs` rewrite (consolidated → peer name, deduped; un-consolidated kept); `Report` contents.
- `internal/hygiene/hygiene_test.go` — dangling ref; empty zone; single-member zone; orphaned alias (incl. the truncated summary); orphaned zone.
- `internal/emitter/brocade/emitter_test.go` — a consolidator-shaped peer zone renders as the expected `zonecreate --peerzone` line (add only if not already covered by B1's tests).
- `internal/converter/converter_test.go` — `--peer-consolidate` e2e on a new `testdata/mds/flat_zones.cfg` (several `ESXn_TGTm` flat zones + one `_SRDF`-style + one 3-member): output has `zonecreate --peerzone "TGT1_peerzone" -principal "TGT1" -members "ESX1;ESX2"`, the SRDF/3-member zones stay flat, `cfgcreate` references the peer zones, stderr has the summary line; `--consolidate-report <tmp>` writes a non-empty report; without `--peer-consolidate` output unchanged; a dangling-ref fixture produces the hygiene warning.
- Manual: `make check` green; the 4 customer captures with `--peer-consolidate` (record before/after zone counts, spot-check one peer zone); without the flag, byte-identical to current `maincd` (only the new hygiene warnings differ on stderr).

## Branch

`feat/peer-consolidate` off `maincd` (Group A + B1 are merged).
