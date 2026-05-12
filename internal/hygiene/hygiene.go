// Package hygiene provides basic static config-hygiene checks: flags zone members
// pointing at undefined aliases, empty/single-member zones, aliases defined but
// unused, and zones not in any zoneset. All findings are appended to cfg.Warnings;
// nothing is fatal. Note: this can't detect physically unconnected devices — there's
// no live fabric data — only static config issues.
package hygiene

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
)

// Check runs the static config-hygiene checks over cfg and appends any findings
// to cfg.Warnings. It does not modify zones, aliases, or configs. Safe to call
// for any direction; safe on an empty cfg.
func Check(cfg *ir.ZoningConfig) {
	checkDanglingAliasRefs(cfg)
	checkEmptyZones(cfg)
	checkSingleMemberZones(cfg)
	checkOrphanedAliases(cfg)
	checkOrphanedZones(cfg)
}

// checkDanglingAliasRefs emits one warning per zone member whose Type is "alias"
// but whose Value is not a key in cfg.Aliases.
func checkDanglingAliasRefs(cfg *ir.ZoningConfig) {
	// Iterate zones in deterministic (sorted) order.
	keys := make([]string, 0, len(cfg.Zones))
	for k := range cfg.Zones {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		z := cfg.Zones[k]
		for _, m := range z.Members {
			if m.Type != "alias" {
				continue
			}
			if _, ok := cfg.Aliases[m.Value]; !ok {
				cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
					"zone %q: member alias %q is not defined — dangling reference",
					z.Name, m.Value,
				))
			}
		}
	}
}

// checkEmptyZones emits a warning for each zone with no members.
func checkEmptyZones(cfg *ir.ZoningConfig) {
	keys := make([]string, 0, len(cfg.Zones))
	for k := range cfg.Zones {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		z := cfg.Zones[k]
		if len(z.Members) == 0 {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q has no members", z.Name,
			))
		}
	}
}

// checkSingleMemberZones emits a warning for each zone with exactly one member.
func checkSingleMemberZones(cfg *ir.ZoningConfig) {
	keys := make([]string, 0, len(cfg.Zones))
	for k := range cfg.Zones {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		z := cfg.Zones[k]
		if len(z.Members) == 1 {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q has a single member — nothing to communicate with", z.Name,
			))
		}
	}
}

// checkOrphanedAliases emits one aggregate warning listing alias names that
// appear in cfg.Aliases but are not referenced by any zone member.
// Emits nothing when every alias is used.
func checkOrphanedAliases(cfg *ir.ZoningConfig) {
	// Build set of alias names referenced by any zone member.
	used := make(map[string]struct{}, len(cfg.Aliases))
	for _, z := range cfg.Zones {
		for _, m := range z.Members {
			if m.Type == "alias" {
				used[m.Value] = struct{}{}
			}
		}
	}

	// Collect unused alias names, then sort.
	unused := make([]string, 0)
	for name := range cfg.Aliases {
		if _, ok := used[name]; !ok {
			unused = append(unused, name)
		}
	}
	if len(unused) == 0 {
		return
	}
	sort.Strings(unused)

	cfg.Warnings = append(cfg.Warnings, buildListWarning(
		fmt.Sprintf("%d alias(es) defined but unused in any zone", len(unused)),
		unused,
	))
}

// checkOrphanedZones emits one aggregate warning listing zone names (deduped)
// that are not referenced by any ZoneConfig's ZoneNames.
// Emits nothing when every zone is in at least one zoneset.
func checkOrphanedZones(cfg *ir.ZoningConfig) {
	// Build set of zone names referenced by any ZoneConfig.
	inConfig := make(map[string]struct{})
	for _, zc := range cfg.ZoneConfigs {
		for _, name := range zc.ZoneNames {
			inConfig[name] = struct{}{}
		}
	}

	// Collect zone names (deduped by .Name, not map key) not in any config.
	orphanedSet := make(map[string]struct{})
	for _, z := range cfg.Zones {
		if _, ok := inConfig[z.Name]; !ok {
			orphanedSet[z.Name] = struct{}{}
		}
	}
	if len(orphanedSet) == 0 {
		return
	}

	orphaned := make([]string, 0, len(orphanedSet))
	for name := range orphanedSet {
		orphaned = append(orphaned, name)
	}
	sort.Strings(orphaned)

	cfg.Warnings = append(cfg.Warnings, buildListWarning(
		fmt.Sprintf("%d zone(s) not in any zoneset", len(orphaned)),
		orphaned,
	))
}

// buildListWarning constructs a warning message with the given prefix and a
// truncated list: up to the first 10 items joined by ", ", followed by
// "(and N more)" if there are more than 10.
func buildListWarning(prefix string, names []string) string {
	const maxShow = 10
	shown := names
	extra := 0
	if len(names) > maxShow {
		shown = names[:maxShow]
		extra = len(names) - maxShow
	}
	msg := prefix + ": " + strings.Join(shown, ", ")
	if extra > 0 {
		msg += fmt.Sprintf(" (and %d more)", extra)
	}
	return msg
}
