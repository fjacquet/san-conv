// Package consolidator collapses flat single-initiator/single-target zones into
// per-target Brocade peer zones. It infers which member is the target and which
// is the initiator by decomposing the zone name (<init>_<target> or
// <target>_<init>) and by cross-zone member frequency (the target alias appears
// in more candidate zones than the initiator). Zones it cannot classify
// confidently are left flat and recorded in Report.Skipped. All mutations are
// applied in-place to the supplied *ir.ZoningConfig.
package consolidator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
)

// PeerZoneSummary describes one peer zone created by Consolidate.
type PeerZoneSummary struct {
	Target      string // the storage-side alias name (the -principal member)
	PeerName    string // the new peer zone's name (Target + "_peerzone")
	VSAN        int
	Members     []string // initiator alias names, in first-seen order
	SourceZones []string // names of the flat zones that were collapsed into this peer zone, sorted
}

// SkippedZone records a 2-member zone Consolidate considered but left flat, and why.
type SkippedZone struct {
	Name   string
	Reason string
}

// Report is the result of a Consolidate call: what was created and what was skipped.
type Report struct {
	PeerZones []PeerZoneSummary // sorted by Target
	Skipped   []SkippedZone     // sorted by Name
}

// consolidatable holds the classification of a single candidate zone.
type consolidatable struct {
	zoneKey string // cfg.Zones key (e.g. "ESX1_TGT1@vsan10")
	zone    *ir.Zone
	init    string
	target  string
}

// Consolidate collapses flat single-initiator/single-target zones in cfg into
// per-target Brocade peer zones, mutating cfg in place, and returns a Report.
// Zones it cannot confidently classify, and zones that aren't 2-member
// alias-membered roleless zones, are left untouched (the 2-member ones are
// recorded in Report.Skipped with a reason).
func Consolidate(cfg *ir.ZoningConfig) Report {
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

		// Name decomposition — try strict then case-insensitive.
		initMember, targetMember, ok := decomposeZoneName(z.Name, a, b)
		if !ok {
			skipped = append(skipped, SkippedZone{
				Name: z.Name,
				Reason: fmt.Sprintf(
					"zone name does not decompose to its two members (%s_%s / %s_%s)",
					a, b, b, a,
				),
			})
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
	// consolidatedNameToPeer maps bare source zone name → peer zone name.
	consolidatedNameToPeer := make(map[string]string)

	var peerZones []PeerZoneSummary

	for _, gk := range groupOrder {
		g := groups[gk]
		sort.Strings(g.sourceZones)

		// Single-initiator group → no value, leave flat.
		if len(g.inits) < 2 {
			for _, srcName := range g.sourceZones {
				skipped = append(skipped, SkippedZone{
					Name: srcName,
					Reason: fmt.Sprintf(
						"only one host zoned to target %q — a peer zone would add no value, left flat",
						gk.target,
					),
				})
			}
			continue
		}

		peerName := gk.target + "_peerzone"

		// Build the peer zone IR.
		pz := &ir.Zone{
			Name: peerName,
			VSAN: gk.vsan,
			Members: []*ir.ZoneMember{
				{Type: "alias", Value: gk.target, Role: "target"},
			},
		}
		for _, init := range g.inits {
			pz.Members = append(pz.Members, &ir.ZoneMember{
				Type:  "alias",
				Value: init,
				Role:  "init",
			})
		}

		// Delete source zones from cfg.
		for _, k := range g.keys {
			delete(cfg.Zones, k)
		}

		// Add peer zone to cfg.
		peerKey := fmt.Sprintf("%s@vsan%d", peerName, gk.vsan)
		cfg.Zones[peerKey] = pz

		// Record the mapping for ZoneConfigs rewrite.
		for _, srcName := range g.sourceZones {
			consolidatedNameToPeer[srcName] = peerName
		}

		peerZones = append(peerZones, PeerZoneSummary{
			Target:      gk.target,
			PeerName:    peerName,
			VSAN:        gk.vsan,
			Members:     g.inits,
			SourceZones: g.sourceZones,
		})
	}

	// Rewrite ZoneConfigs: replace consolidated zone names with peer names, dedup.
	for _, zc := range cfg.ZoneConfigs {
		var newNames []string
		seen := make(map[string]bool)
		for _, name := range zc.ZoneNames {
			mapped := name
			if peer, ok := consolidatedNameToPeer[name]; ok {
				mapped = peer
			}
			if !seen[mapped] {
				seen[mapped] = true
				newNames = append(newNames, mapped)
			}
		}
		zc.ZoneNames = newNames
	}

	// ── Step 6: build sorted report ──────────────────────────────────────────
	sort.Slice(peerZones, func(i, j int) bool {
		return peerZones[i].Target < peerZones[j].Target
	})
	sort.Slice(skipped, func(i, j int) bool {
		return skipped[i].Name < skipped[j].Name
	})

	return Report{
		PeerZones: peerZones,
		Skipped:   skipped,
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

// decomposeZoneName tries to match zoneName == a+"_"+b or zoneName == b+"_"+a,
// first strictly, then case-insensitively. Returns (init, target, true) on
// success where the concatenation order is init+"_"+target.
func decomposeZoneName(zoneName, a, b string) (init, target string, ok bool) {
	// Strict match.
	strictAB := (zoneName == a+"_"+b)
	strictBA := (zoneName == b+"_"+a)
	if strictAB && !strictBA {
		return a, b, true
	}
	if strictBA && !strictAB {
		return b, a, true
	}
	// Both strict match: only possible if a == b (already excluded upstream).
	if strictAB && strictBA {
		return "", "", false
	}

	// Case-insensitive match.
	lower := strings.ToLower(zoneName)
	foldAB := (lower == strings.ToLower(a+"_"+b))
	foldBA := (lower == strings.ToLower(b+"_"+a))
	if foldAB && !foldBA {
		return a, b, true
	}
	if foldBA && !foldAB {
		return b, a, true
	}

	return "", "", false
}
