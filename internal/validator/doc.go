// Package validator implements name sanitization and post-sanitization collision
// detection for IR structs before emitters run.
// It sanitizes names in the IR to conform to Brocade FOS naming rules, rebuilding
// map keys and updating cross-references. Non-fatal diagnostics are appended to
// cfg.Warnings.
// Implemented in Phase 4.
package validator
