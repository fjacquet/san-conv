package validator

import (
	"fmt"
	"regexp"
	"sort"

	"github.com/fjacquet/san-conv/internal/ir"
)

// Package-level compiled regexes (per codebase pattern — compile once, reuse).
var (
	// reInvalidConservative matches any character not in [A-Za-z0-9_].
	// Used for FOS versions prior to 8.1.
	reInvalidConservative = regexp.MustCompile(`[^A-Za-z0-9_]`)

	// reInvalidExtended matches any character not in [A-Za-z0-9_$^-].
	// Used for FOS 8.1+ which permits dollar, caret, and hyphen.
	reInvalidExtended = regexp.MustCompile(`[^A-Za-z0-9_$^-]`)
)

const maxNameLen = 63

// Sanitize applies FOS naming rules to all names in cfg, rebuilding map keys and
// updating cross-references (zone member alias values and zoneset zone names).
// Non-fatal diagnostics are appended to cfg.Warnings.
// Operations are applied in this order:
//  1. Character replacement (invalid chars → "_")
//  2. Truncation (> 63 chars → 63 chars)
//  3. Collision detection and disambiguation ("_2", "_3" suffixes)
//  4. Cross-reference updates (zone member alias values, zoneset zone names)
//  5. Map key rebuilding
func Sanitize(cfg *ir.ZoningConfig, fosVersion string) *ir.ZoningConfig {
	re := selectRegex(fosVersion)

	// Build rename maps for each entity type. Warnings are appended to cfg.
	aliasRename := buildRenameMap(extractAliasNames(cfg.Aliases), re, "alias", cfg)
	zoneRename := buildRenameMap(extractZoneNames(cfg.Zones), re, "zone", cfg)
	cfgRename := buildRenameMap(extractZoneConfigNames(cfg.ZoneConfigs), re, "cfg", cfg)

	// Update zone member alias cross-references before rebuilding the alias map.
	updateZoneMemberAliasRefs(cfg.Zones, aliasRename)

	// Update zoneset zone name cross-references before rebuilding the zone map.
	updateZoneConfigZoneRefs(cfg.ZoneConfigs, zoneRename)

	// Rebuild all three maps with new keys and updated .Name fields.
	cfg.Aliases = rebuildAliasMap(cfg.Aliases, aliasRename)
	cfg.Zones = rebuildZoneMap(cfg.Zones, zoneRename, cfg.SourceFormat)
	cfg.ZoneConfigs = rebuildZoneConfigMap(cfg.ZoneConfigs, cfgRename, cfg.SourceFormat)

	return cfg
}

// selectRegex returns the appropriate compiled regex for the given FOS version.
func selectRegex(fosVersion string) *regexp.Regexp {
	if fosVersion == "8.1+" {
		return reInvalidExtended
	}
	return reInvalidConservative
}

// extractAliasNames returns the set of alias names from the aliases map.
func extractAliasNames(aliases map[string]*ir.Alias) []string {
	names := make([]string, 0, len(aliases))
	for name := range aliases {
		names = append(names, name)
	}
	return names
}

// extractZoneNames returns the set of zone names from the zones map.
// For MDS configs the key is "name@vsanN"; we extract just the .Name field.
func extractZoneNames(zones map[string]*ir.Zone) []string {
	names := make([]string, 0, len(zones))
	for _, z := range zones {
		names = append(names, z.Name)
	}
	return names
}

// extractZoneConfigNames returns the set of zone config names from the map.
func extractZoneConfigNames(cfgs map[string]*ir.ZoneConfig) []string {
	names := make([]string, 0, len(cfgs))
	for _, zc := range cfgs {
		names = append(names, zc.Name)
	}
	return names
}

// buildRenameMap constructs a map[originalName]newName for a slice of entity names.
// It applies char replacement then truncation, detects collisions, and appends
// warnings to cfg for each transformation.
func buildRenameMap(names []string, re *regexp.Regexp, entityType string, cfg *ir.ZoningConfig) map[string]string {
	// Phase 1: apply char replacement and truncation to get the intermediate sanitized name.
	// intermediate[originalName] = sanitizedName (before collision resolution)
	intermediate := make(map[string]string, len(names))
	for _, original := range names {
		sanitized := original

		// Step 1: Character replacement.
		replaced := re.ReplaceAllString(sanitized, "_")
		if replaced != sanitized {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"%s %q: invalid characters replaced -> %q", entityType, original, replaced,
			))
			sanitized = replaced
		}

		// Step 2: Truncation.
		if len(sanitized) > maxNameLen {
			truncated := sanitized[:maxNameLen]
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"%s %q truncated to 63 characters: %q -> %q", entityType, original, sanitized, truncated,
			))
			sanitized = truncated
		}

		intermediate[original] = sanitized
	}

	// Phase 2: detect collisions on the sanitized names.
	// collisions[sanitizedName] = []originalName
	collisions := make(map[string][]string, len(intermediate))
	for original, sanitized := range intermediate {
		collisions[sanitized] = append(collisions[sanitized], original)
	}

	// Phase 3: build the final rename map, disambiguating collisions.
	rename := make(map[string]string, len(names))
	for sanitized, originals := range collisions {
		if len(originals) == 1 {
			// No collision — direct mapping.
			rename[originals[0]] = sanitized
			continue
		}

		// Sort for deterministic assignment order.
		sort.Strings(originals)

		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"collision: names %v all sanitize to %q -- disambiguated", originals, sanitized,
		))

		// First original keeps the sanitized name.
		rename[originals[0]] = sanitized

		// Subsequent originals get _2, _3, etc.
		for i, original := range originals[1:] {
			suffix := fmt.Sprintf("_%d", i+2)
			newName := applyDisambiguatingSuffix(sanitized, suffix)
			rename[original] = newName
		}
	}

	return rename
}

// applyDisambiguatingSuffix appends suffix to base, truncating base first if needed
// to ensure the result stays within maxNameLen characters.
func applyDisambiguatingSuffix(base, suffix string) string {
	if len(base)+len(suffix) > maxNameLen {
		base = base[:maxNameLen-len(suffix)]
	}
	return base + suffix
}

// updateZoneMemberAliasRefs updates ZoneMember.Value for alias-type members
// using the aliasRename map.
func updateZoneMemberAliasRefs(zones map[string]*ir.Zone, aliasRename map[string]string) {
	for _, zone := range zones {
		for _, member := range zone.Members {
			if member.Type == "alias" {
				if newName, ok := aliasRename[member.Value]; ok {
					member.Value = newName
				}
			}
		}
	}
}

// updateZoneConfigZoneRefs updates ZoneConfig.ZoneNames entries using the
// zoneRename map.
func updateZoneConfigZoneRefs(cfgs map[string]*ir.ZoneConfig, zoneRename map[string]string) {
	for _, zc := range cfgs {
		for i, zoneName := range zc.ZoneNames {
			if newName, ok := zoneRename[zoneName]; ok {
				zc.ZoneNames[i] = newName
			}
		}
	}
}

// rebuildAliasMap creates a new map with sanitized keys and updated .Name fields.
func rebuildAliasMap(aliases map[string]*ir.Alias, rename map[string]string) map[string]*ir.Alias {
	newAliases := make(map[string]*ir.Alias, len(aliases))
	for originalKey, alias := range aliases {
		newName, ok := rename[originalKey]
		if !ok {
			// No rename entry means no change.
			newName = originalKey
		}
		alias.Name = newName
		newAliases[newName] = alias
	}
	return newAliases
}

// rebuildZoneMap creates a new map with sanitized keys and updated .Name fields.
// For MDS configs (sourceFormat == "mds-nxos"), the map key is "name@vsanN".
func rebuildZoneMap(zones map[string]*ir.Zone, rename map[string]string, sourceFormat string) map[string]*ir.Zone {
	newZones := make(map[string]*ir.Zone, len(zones))
	for _, zone := range zones {
		originalName := zone.Name
		newName, ok := rename[originalName]
		if !ok {
			newName = originalName
		}
		zone.Name = newName

		var key string
		if sourceFormat == "mds-nxos" {
			key = fmt.Sprintf("%s@vsan%d", newName, zone.VSAN)
		} else {
			key = newName
		}
		newZones[key] = zone
	}
	return newZones
}

// rebuildZoneConfigMap creates a new map with sanitized keys and updated .Name fields.
// For MDS configs, the map key is "name@vsanN".
func rebuildZoneConfigMap(cfgs map[string]*ir.ZoneConfig, rename map[string]string, sourceFormat string) map[string]*ir.ZoneConfig {
	newCfgs := make(map[string]*ir.ZoneConfig, len(cfgs))
	for _, zc := range cfgs {
		originalName := zc.Name
		newName, ok := rename[originalName]
		if !ok {
			newName = originalName
		}
		zc.Name = newName

		var key string
		if sourceFormat == "mds-nxos" {
			key = fmt.Sprintf("%s@vsan%d", newName, zc.VSAN)
		} else {
			key = newName
		}
		newCfgs[key] = zc
	}
	return newCfgs
}

// sanitizeName applies character replacement only (no truncation).
// Exported for potential reuse; kept separate for clarity.
func sanitizeName(name string, re *regexp.Regexp) string {
	return re.ReplaceAllString(name, "_")
}

// truncateName truncates name to maxNameLen if needed.
func truncateName(name string) string {
	if len(name) > maxNameLen {
		return name[:maxNameLen]
	}
	return name
}

// disambiguate returns an oldName -> finalName map for all collision groups.
// Input: collisions map[sanitizedName][]originalName.
// Does not append warnings (caller handles that).
func disambiguate(collisions map[string][]string) map[string]string {
	result := make(map[string]string)
	for sanitized, originals := range collisions {
		sort.Strings(originals)
		result[originals[0]] = sanitized
		for i, original := range originals[1:] {
			suffix := fmt.Sprintf("_%d", i+2)
			result[original] = applyDisambiguatingSuffix(sanitized, suffix)
		}
	}
	return result
}

// Ensure unexported helpers are referenced to satisfy linter.
var _ = sanitizeName
var _ = truncateName
var _ = disambiguate
