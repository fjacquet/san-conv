# ADR-0010: Brocade peer zones round-trip to/from MDS smart zoning

**Date:** 2026-05-12
**Status:** Accepted (resolves the "deferred" item in ADR-0008's Consequences)

## Context

ADR-0008 (Group B1) added `mds2brocade` smart-zoning→peer-zoning: any MDS
zone with `init`/`target`/`both` role tags is converted to a Brocade
`zonecreate --peerzone` command, and role-bearing `ir.ZoneMember` structs flow
through the IR. However, the reverse direction was explicitly deferred:

> `brocade2mds` is unaffected: the Brocade parser sets `Role == ""` for all
> members; parsing `zonecreate --peerzone` back into MDS smart zoning is a
> separate concern (deferred — see ADR for Group B2).

ADR-0009's `--peer-consolidate` flag further widens the gap: it produces
`zonecreate --peerzone` output from flat MDS configs, making it more likely
that an operator will feed a tool-generated Brocade config back through
`brocade2mds`. In both cases the parser silently dropped `--peerzone` lines
(they did not match the `reZoneCreate` regex) and the MDS emitter ignored
`ir.ZoneMember.Role`, so the round-trip was lossy in both directions.

Two failure modes existed:

1. **CLI `zonecreate --peerzone` lines silently dropped.** A Brocade script
   produced by `mds2brocade` or `--peer-consolidate` would produce zero zones
   when fed back through `brocade2mds`.

2. **`ir.ZoneMember.Role` ignored by the MDS emitter.** Even if a peer zone
   did somehow reach the emitter, the output was plain `member pwwn …` lines
   without role keywords and without `zone smart-zoning enable vsan N`, making
   the NX-OS config non-functional as smart zoning.

## Decision

Close both halves of the round-trip:

### Brocade parser (`internal/parser/brocade/parser.go`)

1. **CLI `zonecreate --peerzone` form.** A new regex `reZoneCreatePeer`
   matches `zonecreate --peerzone "name" -principal "…" [-members "…"]`.
   It is tested **before** the existing `reZoneCreate` branch (peer zone lines
   never match the plain form). `-principal` members are stored with
   `Role:"target"`, `-members` members with `Role:"init"`.

2. **cfgshow property-member form.** Brocade cfgshow output represents peer
   zones with a "property member" WWN as the first zone member:
   `00:02:00:00:NN:NN:00:00` where `NN:NN` (big-endian 16-bit) encodes the
   number of principal members. A new post-pass `resolvePeerZoneMarkers` runs
   at the end of `parseCfgshowFormat`; it drops the marker, assigns
   `Role:"target"` to the first N real members, and `Role:"init"` to the rest.
   Decode is best-effort: if the encoded principal count is ≤ 0 or exceeds the
   number of real members, the marker is dropped, the zone is left as a plain
   zone (all roles empty), and a warning is appended.

### MDS emitter (`internal/emitter/mds/emitter.go`)

1. **`zone smart-zoning enable vsan N`.** Before the per-zone loop, a first
   pass collects the set of VSANs that have at least one non-skipped zone with
   a roled member. For each such VSAN, `zone smart-zoning enable vsan N` is
   emitted (sorted ascending) followed by a blank line. NX-OS requires this
   per-VSAN command for role keywords to take effect.

2. **Role suffix on member lines.** In the per-zone member loop, any member
   with a non-empty `Role` is emitted as `member <type> <value> <role>`; plain
   members (empty `Role`) continue to emit `member <type> <value>` — identical
   to before.

## Rationale

- **Faithful mapping.** A Brocade peer zone's `-principal` members are exactly
  the `target` devices in the corresponding MDS smart zone; non-principal
  `-members` are the `init` devices. The mapping is bijective and lossless for
  the `init`/`target` roles.

- **`both` round-trip.** Cisco `both` has no peer-zone equivalent and never
  arises going Brocade→Cisco. In the `mds2brocade` direction (ADR-0008),
  `both` is mapped to `-principal` (connectivity-safe). Therefore, in the
  round-trip `mds2brocade`→`brocade2mds`, a `both` member becomes a
  `-principal` in the Brocade file, then comes back as `target`. The semantic
  loss (`both`→`target`) is acceptable: the round-trip is exact for `init` and
  `target`; the `both` case is documented.

- **`zone smart-zoning enable vsan N` is required.** NX-OS ignores role
  keywords on `member` lines unless smart zoning is explicitly enabled for the
  VSAN. Emitting the enable command makes the output functional out of the box.
  Per-VSAN scoping (not the global `system default zone smart-zoning enable`)
  is used because it matches the specificity of the zone commands that follow
  and avoids unintended fabric-wide side effects.

- **cfgshow marker layout.** The `00:02:00:00:NN:NN:00:00` encoding is not
  publicly documented in Brocade release notes; it is derived from community
  captures. Hence the specific regex and defensive fallback: an unexpected
  count causes the marker to be silently dropped and the zone emitted as plain,
  with a warning, rather than producing incorrect role assignments.

- **No new CLI flag.** Peer-zone parsing and smart-zoning emission are driven
  solely by the data in the input file. The tool has always aimed at faithful
  representation, not opt-in fidelity.

## Consequences

- `brocade2mds` no longer drops `zonecreate --peerzone` input; it produces
  MDS smart-zoning output for any peer zone in the input.
- The MDS emitter emits smart-zoning output (role suffixes,
  `zone smart-zoning enable vsan N`) for the first time.
- `mds2brocade`→`brocade2mds` round-trips peer zones / smart zones faithfully
  for `init` and `target`; `both` becomes `target` after one round-trip
  (documented above).
- The Brocade parser gains `reZoneCreatePeer`, `rePeerMarker`, and
  `resolvePeerZoneMarkers`; imports `fmt` and `strconv`.
- The MDS emitter gains the smart-zoning-enable pre-pass and the role suffix
  in the member loop. Plain zones and configs with no peer zones are emitted
  byte-identically to before.
- **Non-goals:** the lossy `both`→principal direction stays as ADR-0008
  specified. The cfgshow marker layout will be re-validated against a real
  switch capture if one becomes available; the fallback path ensures safety
  until then.
