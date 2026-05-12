# Design: MDS Smart Zoning → Brocade Peer Zoning (Group B1)

- **Date:** 2026-05-12
- **Status:** Approved (brainstorming) — pending implementation plan
- **Author:** Frederic Jacquet (with Claude)
- **Scope tag:** Group B1. Flat-zone *consolidation* (collapsing single-initiator/single-target zones into per-target peer zones, with target/initiator inference) is **Group B2** — a separate spec/plan cycle.
- **Companion ADR:** `docs/adr/0008-mds-smart-zoning-to-peer-zoning.md`

## Background

Cisco MDS "smart zoning" lets a zone member carry a role — `init`, `target`, or
`both`:

```
zone name SmartZone vsan 10
  member pwwn 50:05:0c:00:00:c8:aa:50 init
  member pwwn 50:06:0e:80:04:7c:00:01 target
  member device-alias Array-Port-A target
```

The semantics: within the zone, initiators may talk to targets (and to `both`
devices), but initiators may not talk to other initiators. This collapses the
old N×M "one initiator + one target" zone explosion into a single zone.

**Brocade FOS has the direct equivalent — a (regular) *peer zone*:**

```
zonecreate --peerzone "SmartZone", -principal "50:06:0e:80:04:7c:00:01;…" -members "50:05:0c:00:00:c8:aa:50;…"
```

— *principal* members talk to everyone in the zone; *non-principal* (`-members`)
talk only to principals. (This is distinct from a *Target-Driven Peer Zone*,
which a storage device creates in-band via SCSI and is **not** creatable via
`zonecreate` — out of scope, we never touch those.) Peer zoning has shipped in
Fabric OS since **7.4 (2015)**; current FOS is 10.x.

**Today the tool throws this away.** `internal/parser/mds/parser.go`'s
`processZoneMember` detects a trailing role on `member pwwn X <role>` lines
only, *discards* the role, adds the pWWN as a plain member, and emits
`WARN: zone "Z": smart-zoning role "target" on member X stripped — no FOS
equivalent`. That warning is wrong: there *is* a FOS equivalent. (ADR-0002's
Context lists `init`/`target`/`both` among "constructs that have no FOS
equivalent" — B1 corrects that, and ADR-0008 records the correction.)

None of the four customer captures from Group A use smart zoning, so this
feature is correctness/completeness work rather than a fix for an observed bug —
but it is also the prerequisite emitter path that Group B2 (consolidation) will
build on.

## Goals

- An MDS zone with smart-zoning roles converts to a Brocade peer zone
  (`zonecreate --peerzone … -principal … -members …`), not a flattened plain
  zone, and **without** the misleading "no FOS equivalent" warning.
- Roles are recognised on `member pwwn`, `member device-alias`, and
  `member fcalias` lines (today only `member pwwn`).
- The mapping is unambiguous and connectivity-preserving:
  `target` → principal, `both` → principal, unroled-but-in-a-smart-zoned-zone →
  principal (each with an informational warning for `both`/unroled), `init` →
  non-principal.
- A zone with no roled members behaves exactly as today (plain `zonecreate`).
- ADR-0008 records the decision; USER_GUIDE.md mentions peer-zone output.
- Tests lock all of the above against regression.

## Non-Goals (YAGNI)

- **Flat-zone consolidation** — collapsing existing single-initiator/
  single-target zones into per-target peer zones by *inferring* which member is
  the target. That is Group B2, with its own spec and ADR.
- **`--fos-version` gating of peer-zone output.** Peer zoning is FOS ≥ 7.4
  (2015); the `--fos-version pre-8.1` value exists for the 8.1 *naming-rule*
  change, not for peer-zone support, and an `8.0.x` switch supports peer zones
  fine. Peer-zone output is therefore emitted regardless of `--fos-version`;
  the FOS ≥ 7.4 requirement is noted in ADR-0008 and the user guide. (Whether
  `--fos-version pre-8.1` should be deprecated altogether is a separate
  decision, not B1's.)
- **`brocade2mds` parsing of `zonecreate --peerzone …`** back into MDS smart
  zoning. The Brocade CLI parser today silently skips lines it doesn't
  recognise, so a B1-generated script round-tripped through `brocade2mds` would
  drop the peer zones. This round-trip gap is real but is left for Group B2 or a
  small follow-up.
- **Target-Driven Peer Zones (TDPZ).** Not creatable via `zonecreate`; never in
  scope.
- **No new CLI flag.** Behaviour is driven entirely by the presence of roles in
  the input.

## Architecture

```
parse (mds) ──► validator.Sanitize ──► brocadeemitter.Emit
   │ records member roles    │ unchanged       │ zone has ≥1 roled member?
   │ on ir.ZoneMember.Role   │ (roles ignored) │   yes → zonecreate --peerzone
   │ no more "no FOS" warn   │                 │   no  → zonecreate (plain)
```

Only the **parser** (capture roles, drop the warning), the **IR** (a `Role`
field), and the **Brocade emitter** (render peer zones) change. `validator`
(the sanitizer) and `converter.Run` are untouched — peer-zone output is not
version-gated, so there is nothing for the FOS-version-aware sanitizer to do
here. (Considered and rejected: passing `fosVersion` into `Emit`, or a separate
`peerzone.Resolve` IR pass — both add surface area for no benefit once gating is
dropped.)

### Changed: `internal/ir/zoningconfig.go`

Add one field to `ZoneMember`:

```go
type ZoneMember struct {
	Type string // "pwwn" | "alias" | "unsupported"
	Value string
	// Role is the Cisco smart-zoning role: "" (none), "init", "target", or
	// "both". Brocade emission maps target/both/"" → principal, init →
	// non-principal. Empty for any non-MDS source and for plain MDS zones.
	Role string
}
```

(The IR stays method-free and dependency-free, per ADR-0004.)

### Changed: `internal/parser/mds/parser.go`

- Replace the single `reMemberPWWNRole` with role-aware member regexes (or
  augment the existing `reMemberPWWN` / `reMemberDeviceAlias` / `reMemberFcAlias`
  matches to capture an optional trailing `(init|target|both)`). The canonical
  form is `^\s+member\s+(pwwn|device-alias|fcalias)\s+(\S+)(?:\s+(init|target|both))?\s*$`.
- In `processZoneMember`: when a member line carries a role, set
  `ZoneMember.Role` accordingly; do **not** append the
  `"smart-zoning role … stripped — no FOS equivalent"` warning anymore.
- Everything else in the parser (state machine, IVR handling, alias resolution,
  multi-VSAN diagnostics from Group A) is unchanged.

### Changed: `internal/emitter/brocade/emitter.go`

In the Zones section, for each zone:

1. Determine whether the zone is *smart-zoned*: any `ZoneMember` with a
   non-empty `Role`.
2. **Plain zone (no roled member):** emit `zonecreate "name", "m1;m2;…"` exactly
   as today (no behaviour change).
3. **Smart-zoned zone:** take each member's emitted value exactly as the plain
   `zonecreate` path does today — the alias *name* for an `alias` member, the
   pWWN for a `pwwn` member, and skip `unsupported` members (FOS peer zones
   accept alias names in `-principal`/`-members` just like a plain `zonecreate`
   member list does). Then partition those values:
   - `Role == "target"` → principal list
   - `Role == "both"` → principal list; append a warning
     `zone "Z": smart-zoning role "both" on member X → emitted as a peer-zone principal`
   - `Role == "init"` → non-principal (`-members`) list
   - `Role == ""` (unroled member inside a smart-zoned zone) → principal list;
     append a warning
     `zone "Z": member X has no smart-zoning role → emitted as a peer-zone principal (over-permissive); review`
   - emit `zonecreate --peerzone "name", -principal "p1;p2;…" -members "m1;m2;…"`;
     omit the ` -members "…"` clause entirely if the non-principal list is empty.
   - **Degenerate cases:** if after resolution the principal list is empty (e.g.
     only `init` members survived), fall back to a plain `zonecreate "name",
     "all;resolved;members"` and warn
     `zone "Z": peer zone has no principal members after resolution — emitted as a plain zone`.
     If *no* members survive resolution at all, the existing "no valid FOS
     members → skipped" path applies unchanged.
4. The `cfgcreate`/`cfgenable`/`cfgsave`/`defzone --noaccess` wrapping
   (script mode) is unchanged — a peer zone is referenced in `cfgcreate` by name
   exactly like any other zone.

Output ordering and the deterministic-sort guarantee (ADR/Group-A behaviour) are
preserved: principal and non-principal lists are emitted in the same order the
members appear on the zone (which matches how plain `zonecreate` member lists are
emitted today).

### New: `docs/adr/0008-mds-smart-zoning-to-peer-zoning.md`

Status "Accepted". Records: the mapping (`target`/`both`/unroled → principal,
`init` → non-principal); that peer-zone output is unconditional (FOS ≥ 7.4
requirement noted, not enforced); that this supersedes the "smart-zoning has no
FOS equivalent" claim in ADR-0002's Context; that it is *not* TDPZ; and that
flat-zone consolidation is deferred to B2.

### Changed: `docs/USER_GUIDE.md`

Add a short subsection: MDS smart-zoned zones (`member … init|target|both`) are
converted to Brocade peer zones (`zonecreate --peerzone`), which require Fabric
OS ≥ 7.4 on the target switch; `both` and unroled members are placed as
principals (with a warning).

## Data Flow

1. `mds2brocade` parses the input; `processZoneMember` records each member's
   role on `ir.ZoneMember.Role`. No "no FOS equivalent" warnings.
2. `validator.Sanitize(cfg, fosVersion)` runs as today (name sanitization only;
   it neither reads nor clears `Role`).
3. `brocadeemitter.Emit` walks the zones: roled zone → `zonecreate --peerzone …`
   with the principal/non-principal partition above (+ `both`/unroled/degenerate
   warnings); unroled zone → `zonecreate …` as today.
4. Warnings and the summary line go to stderr unchanged in mechanism.
   `brocade2mds` is entirely unaffected (it never sets `Role`; the Brocade
   parser produces `Role == ""` for everything).

## Error Handling

Consistent with ADR-0002 ("warn and continue"): every special case — `both`
member, unroled member in a smart-zoned zone, empty-principal fallback to plain
zone, all-members-unresolvable — is a `WARN`, never fatal. No new exit paths.

## Testing

**Fixtures (`testdata/mds/`):**
- Extend or add alongside `smart_zoning.cfg`:
  - a zone with `member pwwn … init` / `… target` (the basic case),
  - a zone with `member device-alias … target` and `member fcalias … init`
    (roles on alias members — new parsing),
  - a zone containing a `member … both`,
  - a zone that mixes a roled member with an unroled member,
  - (the existing plain zones in other fixtures already cover the "no roles →
    no change" case).
- A degenerate fixture: a smart-zoned zone whose only members are `init` (→
  empty principal → plain-zone fallback).

**`internal/parser/mds/parser_test.go`:**
- Roles are captured on `ZoneMember.Role` for pwwn, device-alias, and fcalias
  members.
- A smart-zoned fixture produces **no** `"no FOS equivalent"` / `"stripped"`
  warning.
- A plain zone still yields `Role == ""` on all members.

**`internal/emitter/brocade/emitter_test.go`:**
- Smart-zoned zone → output contains `zonecreate --peerzone "Name", -principal "…" -members "…"` with the targets/`both`/unroled in `-principal` and the inits in `-members`.
- `both` member and unroled member each produce the documented warning.
- Smart-zoned zone with no `init` members → `zonecreate --peerzone "Name", -principal "…"` with no `-members` clause.
- Degenerate (only `init`) → plain `zonecreate "Name", "…"` + the fallback warning.
- A plain zone still emits the plain `zonecreate "Name", "m;m"` form (regression).
- `cfgcreate` still lists the peer zone by name.

**`internal/converter/converter_test.go`:**
- End-to-end `mds2brocade` on a smart-zoned fixture (default `--fos-version`) →
  stdout contains the `--peerzone` line; stderr no longer contains "no FOS
  equivalent".

**Existing tests:** all current tests must still pass. In particular the
existing `smart_zoning.cfg` table case in `parser_test.go` currently asserts
`require.Len(t, cfg.Warnings, 3, "…one per smart-zoning role")` — that
expectation changes to **0 warnings** (roles are now captured, not stripped).
That test will be updated as part of the parser task.

## Implementation Order (for the plan)

1. `ir.ZoneMember.Role` field.
2. MDS parser: role-aware member regexes + capture `Role` + remove the
   "stripped — no FOS equivalent" warning; update the `smart_zoning.cfg` parser
   test (now 0 warnings, roles populated); add alias-member-role fixture +
   tests.
3. Brocade emitter: peer-zone rendering with the partition rules, `both`/unroled
   warnings, and the empty-principal / no-members fallbacks; emitter tests.
4. Converter end-to-end test for `mds2brocade` on a smart-zoned input.
5. `docs/adr/0008-mds-smart-zoning-to-peer-zoning.md`.
6. `docs/USER_GUIDE.md` peer-zone subsection.
7. `make check` green; run the four customer captures to confirm no regression
   (they have no smart zoning, so output should be byte-identical to Group A).

## Branch Base

B1 touches `internal/ir`, `internal/parser/mds`, `internal/emitter/brocade` —
the same files Group A's PR #1 (`feat/real-world-config-robustness` → `maincd`)
changes, and #1 is not yet merged. So B1 is being developed on
`feat/peer-zoning-b1`, branched from `feat/real-world-config-robustness`. Its PR
targets `feat/real-world-config-robustness` until #1 merges, then is rebased
onto `maincd`. (Confirm at PR time.)

## Open Questions

None — design approved 2026-05-12.

## Note on workflow

Produced via the `superpowers:brainstorming` flow at the user's request; the
project `CLAUDE.md` asks that code changes route through a GSD command — the
brainstorming→implementation transition will reconcile this (run the plan under
`superpowers:subagent-driven-development`, as Group A's was, or hand to a GSD
execute command) — to be confirmed with the user.
