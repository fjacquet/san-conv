# ADR-0004: Compiler-pipeline architecture with neutral IR

**Date:** 2026-03-28
**Status:** Accepted

## Context

The tool must translate between two structurally similar but syntactically incompatible config formats. A naive approach would parse MDS line-by-line and emit FOS commands inline. This works for simple cases but fails for:

- Bidirectional conversion (reverse direction requires re-parsing the emitted output)
- Cross-reference resolution (zone members reference aliases defined elsewhere in the file)
- Two-pass requirements (NX-OS enhanced device-alias mode requires full alias database before zone members can be resolved)
- Testability (inline parse-and-emit is untestable at unit level)

## Decision

Use a compiler pipeline architecture:

```
Input file
  → Parser (format-specific)
  → ZoningConfig IR (format-neutral struct)
  → Validator/Sanitizer
  → Emitter (format-specific)
  → Output
```

Each stage is independently testable. The IR is the contract between parsers and emitters.

## IR Definition

```go
type ZoningConfig struct {
    Aliases     map[string]*Alias
    Zones       map[string]*Zone
    ZoneConfigs map[string]*ZoneConfig
    SourceFormat string   // "mds-nxos" or "brocade-fos"
    Warnings    []string
}
```

Map keys: MDS uses composite `"name@vsanN"` keys; Brocade uses plain name keys. The `SourceFormat` field lets emitters adapt without inspecting every object.

## Rationale

- **Independent testability**: Each of the 6 packages (`parser/mds`, `parser/brocade`, `validator`, `emitter/brocade`, `emitter/mds`, `converter`) has its own table-driven tests.
- **Bidirectional symmetry**: The same IR struct is produced by both parsers and consumed by both emitters — no duplication.
- **Two-pass parser support**: The IR accumulates all aliases in pass 1 before zone members are resolved in pass 2.
- **Cross-reference safety**: Sanitizer walks the complete IR and updates all cross-references atomically before emitters run.

## Consequences

- `internal/ir/zoningconfig.go` is the most critical contract in the codebase. Breaking changes require updating all 4 parser/emitter packages.
- The `converter` package (`internal/converter/converter.go`) is the only place that wires the pipeline — it has no business logic of its own.
- `io.Writer` interfaces (not `os.Stdout`) are passed into emitters, making them testable without output capture.
