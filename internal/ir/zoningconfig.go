package ir

// ZoningConfig is the canonical format-neutral representation of a SAN zoning
// configuration. All parsers produce a *ZoningConfig; all emitters consume one.
// The IR is intentionally simple: no methods, no logic, no external dependencies.
type ZoningConfig struct {
	// Aliases maps alias name → Alias (both device-alias and fcalias from MDS;
	// alishow entries from Brocade). Key is the original source name (pre-sanitization).
	Aliases map[string]*Alias

	// Zones maps zone name → Zone, scoped per VSAN.
	// For Brocade (single-fabric, no VSANs), all zones use VSAN 0 as a sentinel.
	Zones map[string]*Zone

	// ZoneConfigs maps cfgname → ZoneConfig (Brocade cfg; MDS zoneset).
	ZoneConfigs map[string]*ZoneConfig

	// SourceFormat identifies the parser that produced this IR.
	// Values: "mds-nxos" | "brocade-fos"
	SourceFormat string

	// Warnings accumulates non-fatal issues discovered during parsing.
	// Parsers append to this slice; the CLI layer prints it to stderr.
	Warnings []string
}

// Alias represents a single named WWN mapping.
// In MDS: device-alias or fcalias entry.
// In Brocade: alicreate entry.
type Alias struct {
	Name string // Original source name (pre-sanitization)
	PWWN string // Port WWN in normalized lowercase colon-hex: "10:00:00:00:c9:12:34:56"
	VSAN int    // VSAN scope (MDS fcalias only); 0 means fabric-wide (device-alias or Brocade)
}

// Zone represents a single zone definition.
// In MDS: zone name X vsan Y block.
// In Brocade: zonecreate zone entry.
type Zone struct {
	Name    string        // Original zone name (pre-sanitization)
	VSAN    int           // VSAN scope; 0 for Brocade (no VSAN concept)
	Members []*ZoneMember // Ordered list of zone members
}

// ZoneMember represents a single member within a zone.
// Members can be raw pWWNs, alias references, or unsupported types.
type ZoneMember struct {
	// Type indicates the member variant:
	//   "pwwn"        — raw pWWN (always resolvable to FOS)
	//   "alias"       — reference to an Alias by name (device-alias or fcalias)
	//   "unsupported" — interface, fcid, ip-address, etc. (skipped with warning)
	Type string

	// Value holds the member value appropriate to Type:
	//   "pwwn":        the pWWN string
	//   "alias":       the alias name
	//   "unsupported": original member string (for warning message)
	Value string

	// Role is the Cisco smart-zoning role on this member: "" (none), "init",
	// "target", or "both". Brocade emission maps target/both/"" → peer-zone
	// principal and init → non-principal. Always "" for non-MDS sources and for
	// plain (non-smart-zoned) MDS zones.
	Role string
}

// ZoneConfig represents a zone set / configuration.
// In MDS: zoneset name X vsan Y.
// In Brocade: cfg (cfgcreate).
type ZoneConfig struct {
	Name      string   // Original cfg/zoneset name (pre-sanitization)
	VSAN      int      // VSAN scope (MDS); 0 for Brocade
	ZoneNames []string // Ordered list of zone names in this config/zoneset
}
