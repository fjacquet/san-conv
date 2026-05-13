// Package consolidator collapses flat single-initiator/single-target zones into
// per-target merged role-bearing zones (rendered by the Brocade emitter as peer
// zones and by the MDS emitter as smart zones). It infers which member is the
// target by the zone name: by default the target is whichever member alias is a
// trailing component of the zone name (e.g. "..._GVAMAX01_FA1D04"), the other
// member being the initiator; in strict mode the zone name must be exactly
// <init>_<target> or <target>_<init>. A cross-zone member-frequency veto then
// rejects classifications where the inferred target appears in fewer zones than
// the inferred initiator, or in too few zones to be worth consolidating. Zones
// it cannot classify confidently are left flat and recorded in Report.Skipped.
// All mutations are applied in-place to the supplied *ir.ZoningConfig.
package consolidator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
)

// ConsolidatedZoneSummary describes one merged role-bearing zone created by Consolidate.
// In the mds2brocade direction the Brocade emitter renders it as a `zonecreate --peerzone …` block;
// in the brocade2mds direction the MDS emitter renders it as a smart zone (members with init/target
// roles plus a `zone smart-zoning enable vsan N` directive).
type ConsolidatedZoneSummary struct {
	Target      string // the storage-side alias name (the -principal / target-role member)
	NewName     string // the new merged zone's name (Target + "_" + suffix)
	VSAN        int
	Members     []string // initiator alias names, in first-seen order
	SourceZones []string // names of the flat zones that were collapsed into this zone, sorted
}

// SkippedZone records a 2-member zone Consolidate considered but left flat, and why.
type SkippedZone struct {
	Name   string
	Reason string
}

// Report is the result of a Consolidate call: what was created and what was skipped.
type Report struct {
	Zones   []ConsolidatedZoneSummary // sorted by Target
	Skipped []SkippedZone             // sorted by Name
}

// consolidatable holds the classification of a single candidate zone.
type consolidatable struct {
	zoneKey string // cfg.Zones key (e.g. "ESX1_TGT1@vsan10")
	zone    *ir.Zone
	init    string
	target  string
}

// Consolidate collapses flat single-initiator/single-target zones in cfg into
// per-target merged role-bearing zones, mutating cfg in place, and returns a
// Report. The merged zone is named gk.target + "_" + nameSuffix (e.g.
// "TGT1_peerzone" for the Brocade direction, "TGT1_smartzone" for the MDS
// direction); the IR contents are direction-agnostic (target role on the
// principal member, init role on each initiator), and the respective emitters
// (Brocade peer zone, MDS smart zone) render them accordingly.
//
// When strict is false (the default), the target is identified as the member
// alias that is a trailing component of the zone name; when strict is true, the
// zone name must be exactly <init>_<target> or <target>_<init>. Zones it cannot
// confidently classify, and zones that aren't 2-member alias-membered roleless
// zones, are left untouched (the 2-member ones are recorded in Report.Skipped
// with a reason).
func Consolidate(cfg *ir.ZoningConfig, strict bool, nameSuffix string) Report {
	// ── Step 1: collect candidate zone keys in deterministic order ────────────
	allKeys := make([]string, 0, len(cfg.Zones))
	for k := range cfg.Zones {
		allKeys = append(allKeys, k)
	}
	sort.Strings(allKeys)

	var candidateKeys []string
	for _, k := range allKeys {
		if isCandidate(cfg.Zones[k]) {
			candidateKeys = append(candidateKeys, k)
		}
	}

	// ── Step 2: build frequency map over candidate zones ─────────────────────
	freq := make(map[string]int)
	for _, k := range candidateKeys {
		z := cfg.Zones[k]
		a := z.Members[0].Value
		b := z.Members[1].Value
		if a == b {
			freq[a]++ // degenerate — count once
		} else {
			freq[a]++
			freq[b]++
		}
	}

	// ── Step 3: classify each candidate zone ─────────────────────────────────
	var toConsolidate []consolidatable
	var skipped []SkippedZone

	for _, k := range candidateKeys {
		z := cfg.Zones[k]
		a := z.Members[0].Value
		b := z.Members[1].Value

		// Degenerate zone: both members are the same alias.
		if a == b {
			skipped = append(skipped, SkippedZone{
				Name:   z.Name,
				Reason: "both members are the same alias",
			})
			continue
		}

		// Name decomposition (trailing-component, or strict).
		initMember, targetMember, ok := decomposeZoneName(z.Name, a, b, strict)
		if !ok {
			var reason string
			switch {
			case strict:
				reason = fmt.Sprintf("zone name is not exactly %s_%s or %s_%s (--consolidate-strict)", a, b, b, a)
			case isTrailComponent(z.Name, a) && isTrailComponent(z.Name, b):
				reason = "both members are a trailing component of the zone name — cannot infer the target"
			default:
				reason = fmt.Sprintf("neither member (%s, %s) is a trailing component of the zone name — cannot infer the target", a, b)
			}
			skipped = append(skipped, SkippedZone{Name: z.Name, Reason: reason})
			continue
		}

		// Frequency veto.
		tgtFreq := freq[targetMember]
		initFreq := freq[initMember]
		if tgtFreq < 2 {
			skipped = append(skipped, SkippedZone{
				Name: z.Name,
				Reason: fmt.Sprintf(
					"inferred target %q appears in only %d candidate zone(s) — nothing to consolidate",
					targetMember, tgtFreq,
				),
			})
			continue
		}
		if tgtFreq < initFreq {
			skipped = append(skipped, SkippedZone{
				Name: z.Name,
				Reason: fmt.Sprintf(
					"inferred target %q (%d zones) appears in fewer zones than the inferred initiator %q (%d zones) — left flat for review",
					targetMember, tgtFreq, initMember, initFreq,
				),
			})
			continue
		}

		toConsolidate = append(toConsolidate, consolidatable{
			zoneKey: k,
			zone:    z,
			init:    initMember,
			target:  targetMember,
		})
	}

	// ── Step 4: group consolidatable zones by (target, VSAN) ─────────────────
	type groupKey struct {
		target string
		vsan   int
	}
	type group struct {
		inits       []string // first-seen order (deduped)
		seenInits   map[string]bool
		sourceZones []string // zone bare names, for report
		keys        []string // cfg.Zones keys to delete
	}

	// Use a slice of keys to preserve insertion order (deterministic since
	// toConsolidate was built from sorted candidateKeys).
	var groupOrder []groupKey
	groups := make(map[groupKey]*group)

	for _, c := range toConsolidate {
		gk := groupKey{target: c.target, vsan: c.zone.VSAN}
		g, exists := groups[gk]
		if !exists {
			g = &group{seenInits: make(map[string]bool)}
			groups[gk] = g
			groupOrder = append(groupOrder, gk)
		}
		if !g.seenInits[c.init] {
			g.inits = append(g.inits, c.init)
			g.seenInits[c.init] = true
		}
		g.sourceZones = append(g.sourceZones, c.zone.Name)
		g.keys = append(g.keys, c.zoneKey)
	}

	// ── Step 5: rewrite the IR ─────────────────────────────────────────────────
	// consolidatedNameToNew maps bare source zone name → merged zone name.
	consolidatedNameToNew := make(map[string]string)

	var zones []ConsolidatedZoneSummary

	for _, gk := range groupOrder {
		g := groups[gk]
		sort.Strings(g.sourceZones)

		// Single-initiator group → no value, leave flat.
		if len(g.inits) < 2 {
			for _, srcName := range g.sourceZones {
				skipped = append(skipped, SkippedZone{
					Name: srcName,
					Reason: fmt.Sprintf(
						"only one host zoned to target %q — a merged zone would add no value, left flat",
						gk.target,
					),
				})
			}
			continue
		}

		newName := gk.target + "_" + nameSuffix

		// Build the merged role-bearing zone.
		mz := &ir.Zone{
			Name: newName,
			VSAN: gk.vsan,
			Members: []*ir.ZoneMember{
				{Type: "alias", Value: gk.target, Role: "target"},
			},
		}
		for _, init := range g.inits {
			mz.Members = append(mz.Members, &ir.ZoneMember{
				Type:  "alias",
				Value: init,
				Role:  "init",
			})
		}

		// Delete source zones from cfg.
		for _, k := range g.keys {
			delete(cfg.Zones, k)
		}

		// Add merged zone to cfg.
		newKey := fmt.Sprintf("%s@vsan%d", newName, gk.vsan)
		cfg.Zones[newKey] = mz

		// Record the mapping for ZoneConfigs rewrite.
		for _, srcName := range g.sourceZones {
			consolidatedNameToNew[srcName] = newName
		}

		zones = append(zones, ConsolidatedZoneSummary{
			Target:      gk.target,
			NewName:     newName,
			VSAN:        gk.vsan,
			Members:     g.inits,
			SourceZones: g.sourceZones,
		})
	}

	// Rewrite ZoneConfigs: replace consolidated zone names with merged names, dedup.
	for _, zc := range cfg.ZoneConfigs {
		var newNames []string
		seen := make(map[string]bool)
		for _, name := range zc.ZoneNames {
			mapped := name
			if merged, ok := consolidatedNameToNew[name]; ok {
				mapped = merged
			}
			if !seen[mapped] {
				seen[mapped] = true
				newNames = append(newNames, mapped)
			}
		}
		zc.ZoneNames = newNames
	}

	// ── Step 6: build sorted report ──────────────────────────────────────────
	sort.Slice(zones, func(i, j int) bool {
		return zones[i].Target < zones[j].Target
	})
	sort.Slice(skipped, func(i, j int) bool {
		return skipped[i].Name < skipped[j].Name
	})

	return Report{
		Zones:   zones,
		Skipped: skipped,
	}
}

// isCandidate reports whether z is a consolidation candidate:
// exactly 2 members, both of type "alias", both with Role == "".
func isCandidate(z *ir.Zone) bool {
	if len(z.Members) != 2 {
		return false
	}
	for _, m := range z.Members {
		if m.Type != "alias" || m.Role != "" {
			return false
		}
	}
	return true
}

// decomposeZoneName infers (initiator, target) for a 2-member zone from its
// name and its two member alias names a, b. Returns ok=false when it can't tell.
//
// strict mode: the zone name must be exactly a+"_"+b or b+"_"+a (case-fold
// equality is tried as a fallback). Anything else → no verdict.
//
// relaxed mode (default): the target is whichever member is a *trailing
// component* of the zone name (see isTrailComponent); the other member is the
// initiator. If exactly one member qualifies → that classification; if both
// (ambiguous) or neither → no verdict. This subsumes the strict <init>_<target>
// case (there, the target is the trailing component and the initiator is not).
func decomposeZoneName(zoneName, a, b string, strict bool) (init, target string, ok bool) {
	if strict {
		strictAB := zoneName == a+"_"+b
		strictBA := zoneName == b+"_"+a
		if strictAB && !strictBA {
			return a, b, true
		}
		if strictBA && !strictAB {
			return b, a, true
		}
		if strictAB && strictBA {
			return "", "", false // only if a == b, excluded upstream
		}
		lower := strings.ToLower(zoneName)
		foldAB := lower == strings.ToLower(a+"_"+b)
		foldBA := lower == strings.ToLower(b+"_"+a)
		if foldAB && !foldBA {
			return a, b, true
		}
		if foldBA && !foldAB {
			return b, a, true
		}
		return "", "", false
	}

	// Relaxed: target = the member that is a trailing component of the name.
	ta := isTrailComponent(zoneName, a)
	tb := isTrailComponent(zoneName, b)
	if ta && !tb {
		return b, a, true // a is the trailing component → a is the target
	}
	if tb && !ta {
		return a, b, true
	}
	return "", "", false // both (ambiguous) or neither
}

// isTrailComponent reports whether comp forms a whole trailing component of
// name: name == comp, or name ends with "_"+comp or "-"+comp. A case-fold
// comparison is tried as a fallback.
func isTrailComponent(name, comp string) bool {
	if name == comp || strings.HasSuffix(name, "_"+comp) || strings.HasSuffix(name, "-"+comp) {
		return true
	}
	ln, lc := strings.ToLower(name), strings.ToLower(comp)
	return ln == lc || strings.HasSuffix(ln, "_"+lc) || strings.HasSuffix(ln, "-"+lc)
}
