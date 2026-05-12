package brocade

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
)

// Emit writes Brocade FOS CLI commands derived from cfg to w.
//
// When scriptMode is false, only the command body is written (alicreate,
// zonecreate, cfgcreate lines). When scriptMode is true, the output is wrapped
// with a "defzone --noaccess" preamble and a "cfgsave" postamble; cfgenable is
// always emitted as a comment and never as an executable line.
//
// Map keys in cfg are sorted alphabetically before iteration so that repeated
// calls on the same IR produce identical output regardless of Go's map
// randomisation.
//
// Any zone whose members are all of type "unsupported" is skipped: a warning is
// appended to cfg.Warnings and the zone is excluded from both the zonecreate
// output and any cfgcreate member lists.
func Emit(cfg *ir.ZoningConfig, w io.Writer, scriptMode bool) error {
	// --- Preamble (script mode only) -----------------------------------------
	if scriptMode {
		fmt.Fprintln(w, "defzone --noaccess")
		fmt.Fprintln(w)
	}

	// --- Aliases section (CONV-01) --------------------------------------------
	aliasKeys := sortedStringKeys(cfg.Aliases)
	if len(aliasKeys) > 0 {
		fmt.Fprintln(w, "# --- Aliases ---")
		for _, key := range aliasKeys {
			alias := cfg.Aliases[key]
			fmt.Fprintf(w, "alicreate \"%s\", \"%s\"\n", alias.Name, alias.PWWN)
		}
		fmt.Fprintln(w)
	}

	// --- Zones section (CONV-02) ----------------------------------------------
	// emittedZones tracks which zones were actually written so cfgcreate can be
	// filtered to exclude skipped zones.
	emittedZones := make(map[string]bool)

	zoneKeys := sortedStringKeys(cfg.Zones)
	if len(zoneKeys) > 0 {
		fmt.Fprintln(w, "# --- Zones ---")
		for _, key := range zoneKeys {
			zone := cfg.Zones[key]

			if zoneHasRole(zone) {
				emitPeerZone(zone, cfg, w, emittedZones)
				continue
			}

			// Plain zone — build valid member list, skipping unsupported members.
			var members []string
			for _, m := range zone.Members {
				if m.Type == "unsupported" {
					continue
				}
				members = append(members, m.Value)
			}
			if len(members) == 0 {
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
					"zone %q has no valid FOS members after filtering unsupported types — skipped",
					zone.Name,
				))
				continue
			}
			fmt.Fprintf(w, "zonecreate \"%s\", \"%s\"\n", zone.Name, strings.Join(members, ";"))
			emittedZones[zone.Name] = true
		}
		fmt.Fprintln(w)
	}

	// --- Configs section (CONV-03) -------------------------------------------
	cfgKeys := sortedStringKeys(cfg.ZoneConfigs)
	if len(cfgKeys) > 0 {
		fmt.Fprintln(w, "# --- Configs ---")
		for _, key := range cfgKeys {
			zc := cfg.ZoneConfigs[key]

			// Filter ZoneNames to only include zones that were actually emitted.
			var filteredZoneNames []string
			for _, zoneName := range zc.ZoneNames {
				if emittedZones[zoneName] {
					filteredZoneNames = append(filteredZoneNames, zoneName)
				}
			}

			// Skip cfgcreate if all referenced zones were skipped.
			if len(filteredZoneNames) == 0 {
				continue
			}

			fmt.Fprintf(w, "cfgcreate \"%s\", \"%s\"\n", zc.Name, strings.Join(filteredZoneNames, ";"))
		}
		fmt.Fprintln(w)
	}

	// --- Postamble (script mode only) ----------------------------------------
	if scriptMode {
		for _, key := range cfgKeys {
			zc := cfg.ZoneConfigs[key]
			fmt.Fprintf(w, "# cfgenable \"%s\"  # Uncomment and run manually after verifying the config\n", zc.Name)
		}
		fmt.Fprintln(w, "cfgsave")
	}

	return nil
}

// zoneHasRole reports whether any member of the zone carries a Cisco
// smart-zoning role — in which case the zone is emitted as a Brocade peer zone.
func zoneHasRole(zone *ir.Zone) bool {
	for _, m := range zone.Members {
		if m.Role != "" {
			return true
		}
	}
	return false
}

// emitPeerZone renders a smart-zoned MDS zone as a Brocade peer zone:
//
//	target/both/(roleless) members → -principal,  init members → -members.
//
// 'both' and roleless members each get an informational warning. If no principal
// members survive, it falls back to a plain zonecreate (with a warning); if no
// members survive at all, the zone is skipped (with the standard warning).
func emitPeerZone(zone *ir.Zone, cfg *ir.ZoningConfig, w io.Writer, emittedZones map[string]bool) {
	var principal, nonPrincipal []string
	for _, m := range zone.Members {
		if m.Type == "unsupported" {
			continue
		}
		switch m.Role {
		case "init":
			nonPrincipal = append(nonPrincipal, m.Value)
		case "target":
			principal = append(principal, m.Value)
		case "both":
			principal = append(principal, m.Value)
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q: smart-zoning role \"both\" on member %s → emitted as a peer-zone principal",
				zone.Name, m.Value))
		default: // "" — roleless member inside a smart-zoned zone
			principal = append(principal, m.Value)
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q: member %s has no smart-zoning role → emitted as a peer-zone principal (over-permissive); review",
				zone.Name, m.Value))
		}
	}

	if len(principal) == 0 && len(nonPrincipal) == 0 {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q has no valid FOS members after filtering unsupported types — skipped",
			zone.Name))
		return
	}
	if len(principal) == 0 {
		// A peer zone requires at least one principal — fall back to a plain zone.
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q: peer zone has no principal members after resolution — emitted as a plain zone",
			zone.Name))
		fmt.Fprintf(w, "zonecreate \"%s\", \"%s\"\n", zone.Name, strings.Join(nonPrincipal, ";"))
		emittedZones[zone.Name] = true
		return
	}

	line := fmt.Sprintf("zonecreate --peerzone \"%s\" -principal \"%s\"", zone.Name, strings.Join(principal, ";"))
	if len(nonPrincipal) > 0 {
		line += fmt.Sprintf(" -members \"%s\"", strings.Join(nonPrincipal, ";"))
	}
	fmt.Fprintln(w, line)
	emittedZones[zone.Name] = true
}

// sortedStringKeys returns the keys of any map[string]V in sorted order.
func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
