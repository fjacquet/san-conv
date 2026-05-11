package mds

import (
	"fmt"
	"io"
	"sort"

	"github.com/fjacquet/san-conv/internal/ir"
)

// Emit writes Cisco MDS NX-OS CLI commands derived from cfg to w.
//
// The output is a complete, paste-ready NX-OS config fragment in canonical order:
//  1. device-alias database block (all alias definitions + device-alias commit)
//  2. Per-zone blocks (zone name X vsan N with member lines)
//  3. Per-zoneset blocks (zoneset name X vsan N) followed by zoneset activate
//
// VSAN 0 is treated as a Brocade-sourced sentinel and resolved to defaultVSAN (1).
// MDS-sourced IR may carry non-zero VSAN values which are passed through as-is.
//
// Any zone whose members are all of type "unsupported" is skipped: a warning is
// appended to cfg.Warnings and the zone is excluded from any zoneset member list.
//
// Map keys in cfg are sorted alphabetically before iteration so that repeated
// calls on the same IR produce identical output regardless of Go's map randomisation.
func Emit(cfg *ir.ZoningConfig, w io.Writer) error {
	const defaultVSAN = 1

	// --- Aliases section (CONV-04) -------------------------------------------
	aliasKeys := sortedStringKeys(cfg.Aliases)
	if len(aliasKeys) > 0 {
		fmt.Fprintln(w, "device-alias database")
		for _, key := range aliasKeys {
			alias := cfg.Aliases[key]
			fmt.Fprintf(w, "  device-alias name %s pwwn %s\n", alias.Name, alias.PWWN)
		}
		fmt.Fprintln(w, "device-alias commit")
		fmt.Fprintln(w)
	}

	// --- Zones section (CONV-05) ----------------------------------------------
	// emittedZones tracks which zones were actually written so the zoneset
	// section can filter out skipped zones.
	emittedZones := make(map[string]bool)

	zoneKeys := sortedStringKeys(cfg.Zones)
	for _, key := range zoneKeys {
		zone := cfg.Zones[key]

		// Resolve VSAN: Brocade sentinel 0 -> defaultVSAN; MDS values pass through.
		vsan := zone.VSAN
		if vsan == 0 {
			vsan = defaultVSAN
		}

		// Check whether any non-unsupported members exist.
		var hasValid bool
		for _, m := range zone.Members {
			if m.Type != "unsupported" {
				hasValid = true
				break
			}
		}

		// Skip zones with no valid NX-OS members; warn and continue.
		if !hasValid {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q has no valid NX-OS members after filtering unsupported types -- skipped",
				zone.Name,
			))
			continue
		}

		// Use zone.Name (struct field) — NEVER the map key (may be name@vsanN).
		fmt.Fprintf(w, "zone name %s vsan %d\n", zone.Name, vsan)
		for _, m := range zone.Members {
			switch m.Type {
			case "alias":
				fmt.Fprintf(w, "  member device-alias %s\n", m.Value)
			case "pwwn":
				fmt.Fprintf(w, "  member pwwn %s\n", m.Value)
				// "unsupported" members are silently skipped — already warned above or at parse time.
			}
		}
		fmt.Fprintln(w)
		emittedZones[zone.Name] = true
	}

	// --- ZoneConfigs / Zonesets section (CONV-06, OUT-03) --------------------
	cfgKeys := sortedStringKeys(cfg.ZoneConfigs)
	for _, key := range cfgKeys {
		zc := cfg.ZoneConfigs[key]

		// Resolve VSAN: Brocade sentinel 0 -> defaultVSAN; MDS values pass through.
		vsan := zc.VSAN
		if vsan == 0 {
			vsan = defaultVSAN
		}

		// Filter ZoneNames to only include zones that were actually emitted.
		var filteredZoneNames []string
		for _, zoneName := range zc.ZoneNames {
			if emittedZones[zoneName] {
				filteredZoneNames = append(filteredZoneNames, zoneName)
			}
		}

		// Skip zoneset if all referenced zones were skipped.
		if len(filteredZoneNames) == 0 {
			continue
		}

		fmt.Fprintf(w, "zoneset name %s vsan %d\n", zc.Name, vsan)
		for _, zoneName := range filteredZoneNames {
			fmt.Fprintf(w, "  member %s\n", zoneName)
		}
		fmt.Fprintln(w)

		// zoneset activate is a REAL config command in NX-OS — never commented out
		// (unlike Brocade's cfgenable which is always emitted as a comment).
		fmt.Fprintf(w, "zoneset activate name %s vsan %d\n", zc.Name, vsan)
		fmt.Fprintln(w)
	}

	return nil
}

// sortedStringKeys returns the keys of any map[string]V in sorted order.
// Each emitter package keeps its own unexported copy — no shared utility package.
func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
