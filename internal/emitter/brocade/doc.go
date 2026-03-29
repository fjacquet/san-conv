// Package brocade implements the Brocade FOS CLI command emitter.
// It consumes a *ir.ZoningConfig and writes alicreate/zonecreate/cfgcreate commands
// to an io.Writer. Includes mandatory defzone --noaccess preamble and cfgsave postamble.
// Implemented in Phase 5.
package brocade
