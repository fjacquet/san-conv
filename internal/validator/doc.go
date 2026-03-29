// Package validator implements name sanitization and post-sanitization collision
// detection for IR structs before emitters run.
// It reads IR and emits []Warning; it never mutates the IR.
// Implemented in Phase 4.
package validator
