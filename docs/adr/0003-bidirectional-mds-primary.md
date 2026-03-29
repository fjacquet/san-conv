# ADR-0003: Bidirectional conversion with MDS→Brocade as primary direction

**Date:** 2026-03-28
**Status:** Accepted

## Context

The immediate use case driving this tool is migrating from Cisco MDS to Brocade switches. However, during design, the reverse direction (Brocade→MDS) was identified as a useful secondary case: back-out plans, hybrid environments, and documentation generation.

## Decision

The tool implements full bidirectional conversion. The MDS→Brocade direction is the primary path and drives the phase ordering. Brocade→MDS is implemented as a symmetric counterpart after the primary direction is proven.

## Rationale

- **MDS→Brocade is the migration driver**: Phase 2 (MDS parser) and Phase 5 (Brocade emitter) are on the critical path.
- **Symmetric IR makes reverse cheap**: Once the IR contract is established, adding the reverse direction requires only a Brocade parser (Phase 3) and an MDS emitter (Phase 6) — no changes to the IR or core pipeline.
- **Back-out value**: If a Brocade-to-MDS back-out is needed post-migration, the tool handles it without manual retranslation.

## Consequences

- The IR (`internal/ir/zoningconfig.go`) must be neutral: no MDS-isms or FOS-isms. `SourceFormat` field tags the origin but the struct is identical for both directions.
- VSAN handling asymmetry: MDS uses per-VSAN zones (map key `name@vsanN`); Brocade uses a single flat namespace. The MDS emitter maps VSAN 0 (Brocade sentinel) to VSAN 1 by default.
- The `--direction` flag selects the pipeline; both subcommand aliases (`mds2brocade`, `brocade2mds`) remain as named entry points.
