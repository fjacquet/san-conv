# ADR-0009: Flat-zone consolidation into peer zones (--peer-consolidate)

**Date:** 2026-05-12
**Status:** Accepted

## Context

Real MDS fabric configs captured from customer environments contain on the order
of 730 zones per fabric, almost entirely **flat single-initiator/single-target
zones** named `<initiator-alias>_<target-alias>` (e.g. `ESX01_HBA0_ArrayPort3`).
Each zone has exactly two members: the initiator alias and the target alias.

Brocade best practice for this topology is **one peer zone per storage port**:

```
zonecreate --peerzone "ArrayPort3_peerzone" -principal "ArrayPort3" -members "ESX01_HBA0;ESX02_HBA0;…"
```

This shrinks hundreds of `zonecreate` lines to one per storage port, is easier
to audit, and is the canonical FOS construct for host-to-target access.

ADR-0008 (Group B1) already built the `zonecreate --peerzone` emitter path:
any `ir.Zone` that carries at least one roled member is rendered as a peer zone.
B1 handles configs that carry **explicit** smart-zoning roles (`init`/`target`/
`both`). The customer configs above carry **no roles** — they are flat,
unstructured zones — so B1's code path is not triggered.

Because the config carries no role information, target vs initiator must be
**inferred** from the zone name and zone-membership statistics. Heuristic
inference can be wrong: misclassifying a storage port as an initiator (or vice
versa) would silently drop storage paths in production. The feature is therefore
**opt-in**, conservative (ambiguous zones are left flat), and ships with a
per-zone report so an engineer can review every inference before applying the
output.

Separately, every conversion should also flag **static config-hygiene problems**
(dangling alias references, empty or single-member zones, unused aliases, zones
not in any zoneset) regardless of whether consolidation is requested. These are
cheap checks on the same IR and are useful on every migration.

## Decision

### --peer-consolidate flag

Add an opt-in `--peer-consolidate` flag to `mds2brocade` (and the bare
`san-conv [input]` root form). Default: **off**. Not available on `brocade2mds`.

When the flag is set, after VSAN filtering and before name sanitization, a
consolidator pass groups eligible flat zones by inferred target and emits one
peer zone per target.

**Candidate criteria** (a zone must meet all four to be eligible):

1. Exactly 2 members
2. Both members are alias references (not raw pWWNs)
3. Neither member carries a smart-zoning role (they would already be handled by
   the B1 path)
4. The zone name decomposes as `<alias-A>_<alias-B>` where `alias-A` and
   `alias-B` are the exact two members (zone-name decomposition)

**Inference heuristic:**

- **Primary (zone-name decomposition):** a zone named `<X>_<Y>` whose two
  members are exactly `X` and `Y` is read as: `X` = initiator, `Y` = target.
  This is deterministic for configs that follow the `<init>_<target>` naming
  convention. Zones whose names do not decompose this way are immediately left
  flat.
- **Frequency veto:** among all candidate zones, count how many zones each alias
  appears in as the inferred target. A zone's inferred target must satisfy:
  `freq(target) ≥ 2` AND `freq(target) ≥ freq(initiator)`. If either condition
  fails the zone is left flat. Rationale: a genuine storage port is shared by
  many hosts; if an alias appears as "target" in only one zone, or appears less
  often than the "initiator," the name decomposition is likely backwards or
  coincidental.
- **Single-host veto:** if the inferred target appears as the target in exactly
  one candidate zone (only one host), that zone is left flat — a peer zone with
  one member adds no benefit over a flat zone.

**Output:**

- Eligible zones are grouped by inferred target. Each group becomes one peer
  zone named `<target>_peerzone` with `-principal "<target>"` and
  `-members "<all-initiators>"`.
- The original flat zones are **removed** from the output; `cfgcreate` is
  rebuilt to reference the peer zones in place of the removed flat zones.
- Name sanitization and collision detection apply to `<target>_peerzone` names
  as they do to all other names.

**Reporting:**

- A summary line on stderr: `Consolidated N flat zones into M peer zones; K zone(s) left flat`
- `--consolidate-report <file>` writes a full report: every peer zone with its
  principal, members, and the source flat zones it replaced; every zone left flat
  and the reason.

**FOS version requirement:** peer-zone output requires Fabric OS ≥ 7.4 on the
target switch (inherited from ADR-0008; the requirement is documented but not
enforced by the tool).

### Always-on config-hygiene warnings

On every conversion (unconditional, not gated on `--peer-consolidate`), the tool
emits `WARN:` lines on stderr for the following static config problems:

- A zone member references an alias that is not defined anywhere — **dangling
  reference**
- A zone has **0 members** (empty zone)
- A zone has **1 member** (nothing to communicate with)
- Aliases defined but never referenced in any zone — reported as one aggregate
  warning
- Zones not referenced by any zoneset — reported as one aggregate warning

These are static checks against the config file. The tool has no view of the
live fabric and cannot detect physically unconnected or logged-out devices.

## Rationale

- **Opt-in and conservative:** the name-decomposition heuristic is accurate for
  configs following the `<init>_<target>` convention but cannot be proven correct
  from the config alone. Misidentifying a storage port as an initiator would
  silently drop production paths after the config is applied. The default-off
  posture, the frequency veto, and the per-zone report collectively ensure an
  engineer reviews every inference before applying the output.
- **Two signals, name first:** the customer data establishes `<init>_<target>`
  naming as a convention, so zone-name decomposition is deterministic and exact
  for the vast majority of zones. The frequency check is a veto, not a
  classifier — a zone whose name does not decompose is left flat rather than
  classified by frequency alone, which prevents false positives in edge cases
  (replication pairs like `RA1_RA2_SRDF`, test zones, and so on).
- **Replace, don't add:** the point of consolidation is to shrink zone count.
  Keeping both the original flat zones and the new peer zones in the output would
  be redundant and confusing. The flat zones are removed; `cfgcreate` is
  rebuilt.
- **Build on B1's emitter:** the consolidator synthesises `ir.Zone` structs with
  `ir.ZoneMember.Role` set (`target` → principal, initiators → non-principal).
  The existing `zonecreate --peerzone` emitter from ADR-0008 renders them. There
  is one peer-zone code path, not two.
- **Hygiene checks for free:** the IR needed for consolidation is the same IR the
  hygiene checks operate on. Emitting these warnings on every run costs
  negligible CPU and catches problems (dangling aliases, orphaned zones) that are
  universally useful to flag before a migration, regardless of whether
  consolidation is requested.

## Consequences

- New `internal/consolidator` package: implements candidate selection, heuristic
  inference, grouping, and report generation.
- New `internal/hygiene` package: implements the five static config checks.
- New flags on `mds2brocade` and the root command (not `brocade2mds`):
  - `--peer-consolidate` (bool, default false)
  - `--consolidate-report <file>` (string, default "")
- `Options` struct gains `Consolidate bool` and `ConsolidateReport string`
  fields.
- `converter.Run` calls `hygiene.Check` unconditionally (after parse, after VSAN
  filter) and calls `consolidator.Consolidate` when `Options.Consolidate` is
  true (after VSAN filtering, before name sanitization).
- The Brocade emitter is unchanged — B1's `emitPeerZone` renders consolidator
  output without modification.
- `brocade2mds` is unaffected.
- USER_GUIDE is updated to document `--peer-consolidate`, `--consolidate-report`,
  the hygiene warnings, and the FOS ≥ 7.4 peer-zone requirement.
- **Non-goals / possible follow-ups:**
  - Consolidation of 3+-member flat zones
  - WWN-OUI or name-prefix heuristics as an alternative inference signal
  - `brocade2mds` parsing of `zonecreate --peerzone` back into MDS smart zoning
    (deferred — see ADR-0008 Consequences)
