# ADR-0005: Two-pass MDS parser for device-alias enhanced mode

**Date:** 2026-03-28
**Status:** Accepted

## Context

Cisco NX-OS MDS has two device-alias modes:

- **Basic mode** (legacy): Zone members are always raw pWWNs (`member pwwn 10:00:...`)
- **Enhanced mode** (default since NX-OS 8.5(1)): Zone members reference alias names directly (`member device-alias HOST_A`)

In enhanced mode, a zone member `member device-alias HOST_A` cannot be resolved until the full `device-alias database` block has been parsed — which appears earlier in the running-config, but the parser may encounter zone blocks before confirming all aliases are loaded.

A single-pass parser that emits IR on the fly would silently drop all enhanced-mode zone members referencing aliases not yet seen, producing an incomplete conversion with no error.

## Decision

The MDS parser uses a two-pass approach:

1. **Pass 1**: Scan the entire file. Collect the full `device-alias database` (global aliases) and all `fcalias` definitions (per-VSAN aliases). Do not process zone blocks.
2. **Pass 2**: Scan the file again. Process `zone name` and `zoneset name` blocks. Resolve `member device-alias` and `member fcalias` references against the maps built in Pass 1.

## Rationale

- **Correctness over performance**: A 10,000-line running-config parses in < 10ms even with two passes. The cost is negligible.
- **No silent data loss**: Every unresolvable alias reference emits a named warning rather than being silently dropped.
- **Handles any ordering**: Some vendors reorder running-config sections; two-pass is robust regardless of section order.

## Consequences

- The parser reads the input twice using `bufio.Scanner` on a `strings.Reader` (file content buffered to string in pass 1).
- Memory: the full file is held in memory during parsing. For typical configs (< 5MB), this is not a concern.
- `member pwwn` entries are resolved immediately in pass 2 without lookup.
- `member interface`, `member fcid`, `member ip-address`: emits an unsupported-type warning and skips the member.
