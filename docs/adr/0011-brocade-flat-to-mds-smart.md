# ADR-0011: brocade2mds --smart-consolidate (flat Brocade → merged MDS smart zones)

**Status:** Accepted
**Date:** 2026-05-13
**Depends on:** ADR-0008 (smart-zoning → peer-zoning emitter, B1), ADR-0009 (mds2brocade flat-zone consolidation, B2), ADR-0010 (peer-zone → smart-zone round-trip, B3)
**Companion plan:** docs/superpowers/plans/2026-05-13-smart-consolidate-brocade-to-mds.md

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
