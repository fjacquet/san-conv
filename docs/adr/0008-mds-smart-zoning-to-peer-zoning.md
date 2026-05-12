# ADR-0008: Map MDS smart zoning to Brocade peer zoning

**Date:** 2026-05-12
**Status:** Accepted (amends ADR-0002's "no FOS equivalent" claim for smart zoning)

## Context

Cisco MDS "smart zoning" lets a zone member carry a role — `init`, `target`, or
`both` (e.g. `member pwwn 50:06:0e:80:04:7c:00:01 target`). Within the zone,
initiators reach targets and `both` devices, but initiators do not reach other
initiators — collapsing the classic N×M single-initiator/single-target zone
explosion into one zone.

Brocade Fabric OS has the direct equivalent: a (regular) **peer zone**, created
with `zonecreate --peerzone "name" -principal "<pwwns>" [-members "<pwwns>"]`.
Principal members communicate with all members of the zone; non-principal
(`-members`) communicate only with principals. Peer zoning has been in Fabric
OS since **7.4 (2015)**; the current release is FOS 10.x.

(A *Target-Driven Peer Zone* — TDPZ — is a different construct, created in-band
by a storage device via SCSI and **not** creatable with `zonecreate`. This tool
never produces or consumes TDPZs.)

Until now the tool discarded smart-zoning roles: `processZoneMember` detected a
trailing role on `member pwwn` lines only, dropped it, added the device as a
plain member, and warned `zone "Z": smart-zoning role "target" on member X
stripped — no FOS equivalent`. ADR-0002's Context likewise lists
`init`/`target`/`both` among "constructs that have no FOS equivalent". That is
incorrect — peer zoning is the equivalent.

## Decision

`mds2brocade` converts a smart-zoned MDS zone to a Brocade **peer zone**:

1. Roles are recognised on `member pwwn`, `member device-alias`, and
   `member fcalias` lines and recorded on the IR (`ir.ZoneMember.Role`).
2. A zone with at least one roled member is emitted as
   `zonecreate --peerzone "name", -principal "<principals>" -members "<non-principals>"`
   (the ` -members "…"` clause is omitted when there are no non-principals).
3. Role → slot mapping:
   - `target` → principal
   - `both` → principal — a `both` device must reach both the targets and the
     initiators in the zone, which only the principal slot provides; this is
     slightly over-permissive versus exact smart-zoning semantics but never
     under-connects. An informational warning is emitted.
   - unroled member inside an otherwise-smart-zoned zone → principal — the
     static config does not carry the role Cisco would auto-detect from FCNS, so
     we choose the connectivity-preserving slot. An informational warning is
     emitted.
   - `init` → non-principal (`-members`)
4. Peer-zone output is **unconditional** — it is *not* gated on `--fos-version`.
   Peer zoning predates the FOS 8.1 naming-rule change that the `pre-8.1` flag
   exists for, and any switch new enough to matter runs FOS ≥ 7.4. The FOS ≥ 7.4
   requirement is documented (USER_GUIDE) but not enforced.
5. A zone with no roled members is emitted exactly as before — a plain
   `zonecreate "name", "m1;m2;…"`. No new CLI flag is introduced; behaviour is
   driven solely by the presence of roles in the input.
6. Degenerate cases fall back, with a warning, rather than producing invalid
   output: a peer zone that would have no principal members → plain `zonecreate`;
   a zone whose members are all unresolvable/`unsupported` → skipped (the
   existing path).

This amends ADR-0002: smart-zoning keywords now *do* have a FOS equivalent and
are converted, not skipped.

## Rationale

- **Fidelity**: the input explicitly marked the zone as smart-zoned. Faithfully
  converting it means producing a peer zone, not silently flattening intent.
- **Connectivity first**: where the static config is ambiguous (`both`, unroled
  members), the over-permissive principal slot is chosen. An over-broad peer
  zone never breaks a path a flat zone would have allowed; an under-broad one
  would. Each such choice is surfaced as a warning so an engineer can tighten it
  if desired.
- **No version gate**: building a "flatten for old FOS" branch in 2026 adds code
  and tests for FOS 7.0–7.3, which is not a realistic migration target. The
  `--fos-version` flag's `pre-8.1` value is about name character sets, not
  peer-zone support.
- **No new flag**: a flag would mean the *default* still loses smart-zoning
  intent. Driving off the input keeps the conversion faithful by default.

## Consequences

- `ir.ZoneMember` gains a `Role string` field (`"" | "init" | "target" | "both"`).
- `internal/parser/mds/parser.go`: role-aware member regexes; `processZoneMember`
  records `Role`; the `"smart-zoning role … stripped — no FOS equivalent"`
  warning is removed. The existing `smart_zoning.cfg` parser test changes from
  asserting 3 warnings to 0.
- `internal/emitter/brocade/emitter.go`: emits `zonecreate --peerzone …` for
  roled zones, with `both`/unroled/empty-principal warnings.
- `validator` (the sanitizer) and `converter.Run` are unchanged.
- `brocade2mds` is unaffected: the Brocade parser sets `Role == ""` for all
  members; parsing `zonecreate --peerzone` back into MDS smart zoning is a
  separate concern (deferred — see ADR for Group B2).
- Flat-zone *consolidation* (inferring target vs initiator to collapse existing
  single-initiator/single-target zones into per-target peer zones) is **not**
  part of this decision — it is Group B2 and will have its own ADR, because the
  inference heuristic carries safety implications a deterministic mapping does
  not.
- USER_GUIDE documents peer-zone output and the FOS ≥ 7.4 requirement.
