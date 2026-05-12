package brocade

import (
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/fjacquet/san-conv/internal/preprocess"
)

// cfgshowState tracks the current parsing context inside parseCfgshowFormat.
type cfgshowState int

const (
	stateIdle  cfgshowState = iota
	stateCfg                // Inside a cfg: block
	stateZone               // Inside a zone: block
	stateAlias              // Inside an alias: block
)

// Compiled regexps for cfgshow and CLI formats.
var (
	// cfgshow section boundaries
	reCfgshowSection   = regexp.MustCompile(`^Defined configuration:\s*$`)
	reCfgshowEffective = regexp.MustCompile(`^Effective configuration:\s*$`)

	// cfgshow type tokens (leading whitespace + keyword + name)
	reCfgToken   = regexp.MustCompile(`^\s+cfg:\s+(\S+)`)
	reZoneToken  = regexp.MustCompile(`^\s+zone:\s+(\S+)`)
	reAliasToken = regexp.MustCompile(`^\s+alias:\s+(\S+)`)

	// CLI format commands
	reAliCreate  = regexp.MustCompile(`^alicreate\s+"([^"]+)"\s*,\s*"([^"]+)"`)
	reZoneCreate = regexp.MustCompile(`^zonecreate\s+"([^"]+)"\s*,\s*"([^"]+)"`)
	reCfgCreate  = regexp.MustCompile(`^cfgcreate\s+"([^"]+)"\s*,\s*"([^"]+)"`)

	// CLI peer zone form: zonecreate --peerzone "name" -principal "..." [-members "..."]
	reZoneCreatePeer = regexp.MustCompile(`^zonecreate\s+--peerzone\s+"([^"]+)"\s+-principal\s+"([^"]*)"(?:\s+-members\s+"([^"]*)")?`)

	// cfgshow peer-zone property member WWN: 00:02:00:00:NN:NN:00:00
	rePeerMarker = regexp.MustCompile(`^00:02:00:00:([0-9a-fA-F]{2}):([0-9a-fA-F]{2}):00:00$`)

	// CLI format detection
	reCLICommand = regexp.MustCompile(`^(alicreate|zonecreate|cfgcreate)\s+`)
)

// Parse reads a Brocade FOS configuration from r (either cfgshow output or CLI
// script format) and returns a populated *ir.ZoningConfig. Auto-detection
// determines which sub-parser to use. Non-fatal issues are appended to
// cfg.Warnings. Parse only returns an error for I/O failures.
func Parse(r io.Reader) (*ir.ZoningConfig, error) {
	lines, err := preprocess.Clean(r)
	if err != nil {
		return nil, err
	}

	cfg := &ir.ZoningConfig{
		SourceFormat: "brocade-fos",
		Aliases:      make(map[string]*ir.Alias),
		Zones:        make(map[string]*ir.Zone),
		ZoneConfigs:  make(map[string]*ir.ZoneConfig),
	}

	if detectCLIFormat(lines) {
		parseCLIFormat(lines, cfg)
	} else {
		parseCfgshowFormat(lines, cfg)
	}

	return cfg, nil
}

// detectCLIFormat scans lines to determine if the input is a FOS CLI script
// (alicreate/zonecreate/cfgcreate commands) vs cfgshow output. Returns true
// for CLI format, false for cfgshow format (the default). (PARSE-09)
func detectCLIFormat(lines []string) bool {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "Defined configuration:" {
			return false
		}
		if reCLICommand.MatchString(trimmed) {
			return true
		}
	}
	return false
}

// parseCfgshowFormat parses Brocade FOS cfgshow output into cfg. It handles
// backslash line continuation and stops parsing at the Effective configuration
// section to avoid duplicate entries. (PARSE-07)
//
//nolint:cyclop,funlen // State machine is inherently complex; splitting would obscure the logic
func parseCfgshowFormat(lines []string, cfg *ir.ZoningConfig) {
	state := stateIdle
	inDefinedSection := false
	continuation := false

	var currentCfg *ir.ZoneConfig
	var currentZone *ir.Zone
	var currentAlias *ir.Alias

	for _, line := range lines {
		// Section boundary detection
		if reCfgshowSection.MatchString(line) {
			inDefinedSection = true
			state = stateIdle
			continuation = false
			continue
		}
		if reCfgshowEffective.MatchString(line) {
			// Stop parsing — everything after Effective configuration is a duplicate
			return
		}

		// Only process lines inside the Defined configuration section
		if !inDefinedSection {
			continue
		}

		// If we are in a continuation, do NOT check type tokens — treat as member data
		if continuation {
			members, cont := parseMemberLine(line)
			continuation = cont
			appendMembers(members, state, currentCfg, currentZone, currentAlias)
			continue
		}

		// Try type token matches first (only when not in continuation)
		if m := reCfgToken.FindStringSubmatch(line); m != nil {
			name := m[1]
			zc := &ir.ZoneConfig{Name: name, VSAN: 0}
			cfg.ZoneConfigs[name] = zc
			currentCfg = zc
			currentZone = nil
			currentAlias = nil
			state = stateCfg
			continuation = false
			continue
		}

		if m := reZoneToken.FindStringSubmatch(line); m != nil {
			name := m[1]
			z := &ir.Zone{Name: name, VSAN: 0}
			cfg.Zones[name] = z
			currentZone = z
			currentCfg = nil
			currentAlias = nil
			state = stateZone
			continuation = false
			continue
		}

		if m := reAliasToken.FindStringSubmatch(line); m != nil {
			name := m[1]
			a := &ir.Alias{Name: name, VSAN: 0}
			cfg.Aliases[name] = a
			currentAlias = a
			currentCfg = nil
			currentZone = nil
			state = stateAlias
			continuation = false
			continue
		}

		// Member line — applies to current state
		if state != stateIdle {
			members, cont := parseMemberLine(line)
			continuation = cont
			appendMembers(members, state, currentCfg, currentZone, currentAlias)
		}
	}

	resolvePeerZoneMarkers(cfg)
}

// resolvePeerZoneMarkers reinterprets cfgshow zones whose first member is a
// Brocade peer-zone "property member" WWN (00:02:00:00:NN:NN:00:00, where
// NN:NN is the principal count, big-endian). The marker is dropped; the first
// <count> remaining members become principals (Role "target"), the rest
// non-principals (Role "init"). If the count doesn't decode sensibly, the
// marker is dropped and the zone is left as a plain zone with a warning.
func resolvePeerZoneMarkers(cfg *ir.ZoningConfig) {
	for key, z := range cfg.Zones {
		if len(z.Members) == 0 || z.Members[0].Type != "pwwn" {
			continue
		}
		mm := rePeerMarker.FindStringSubmatch(z.Members[0].Value)
		if mm == nil {
			continue
		}
		hi, _ := strconv.ParseInt(mm[1], 16, 0)
		lo, _ := strconv.ParseInt(mm[2], 16, 0)
		count := int(hi)*256 + int(lo)
		rest := z.Members[1:] // drop the marker
		if count <= 0 || count > len(rest) {
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q: peer-zone property marker %q did not decode (claims %d principals, zone has %d members) — emitted as a plain zone, review",
				z.Name, z.Members[0].Value, count, len(rest)))
			z.Members = rest
			cfg.Zones[key] = z
			continue
		}
		for i, m := range rest {
			if i < count {
				m.Role = "target"
			} else {
				m.Role = "init"
			}
		}
		z.Members = rest
		cfg.Zones[key] = z
	}
}

// appendMembers dispatches parsed member tokens to the appropriate current
// object based on the current state.
func appendMembers(members []string, state cfgshowState, currentCfg *ir.ZoneConfig, currentZone *ir.Zone, currentAlias *ir.Alias) {
	switch state {
	case stateCfg:
		if currentCfg != nil {
			currentCfg.ZoneNames = append(currentCfg.ZoneNames, members...)
		}
	case stateZone:
		if currentZone != nil {
			for _, m := range members {
				if looksLikeWWN(m) {
					currentZone.Members = append(currentZone.Members, &ir.ZoneMember{
						Type:  "pwwn",
						Value: normalizeWWN(m),
					})
				} else {
					currentZone.Members = append(currentZone.Members, &ir.ZoneMember{
						Type:  "alias",
						Value: m,
					})
				}
			}
		}
	case stateAlias:
		// Alias has exactly one pWWN — only take the first member
		if currentAlias != nil && currentAlias.PWWN == "" && len(members) > 0 {
			currentAlias.PWWN = normalizeWWN(members[0])
		}
	default:
		// stateIdle: nothing to do
	}
}

// parseCLIFormat parses a FOS CLI script (alicreate/zonecreate/cfgcreate
// commands) into cfg. Lines not matching any known pattern are silently
// skipped. (PARSE-08)
func parseCLIFormat(lines []string, cfg *ir.ZoningConfig) {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if m := reAliCreate.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			pwwn := normalizeWWN(m[2])
			cfg.Aliases[name] = &ir.Alias{Name: name, PWWN: pwwn, VSAN: 0}
			continue
		}

		if m := reZoneCreatePeer.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			z := &ir.Zone{Name: name, VSAN: 0}
			addPeerMembers := func(list, role string) {
				for _, raw := range strings.Split(list, ";") {
					tok := strings.TrimSpace(raw)
					if tok == "" {
						continue
					}
					if looksLikeWWN(tok) {
						z.Members = append(z.Members, &ir.ZoneMember{Type: "pwwn", Value: normalizeWWN(tok), Role: role})
					} else {
						z.Members = append(z.Members, &ir.ZoneMember{Type: "alias", Value: tok, Role: role})
					}
				}
			}
			addPeerMembers(m[2], "target") // -principal
			addPeerMembers(m[3], "init")   // -members (m[3] is "" if the clause is absent)
			cfg.Zones[name] = z
			continue
		}

		if m := reZoneCreate.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			memberList := m[2]
			z := &ir.Zone{Name: name, VSAN: 0}
			for _, raw := range strings.Split(memberList, ";") {
				member := strings.TrimSpace(raw)
				if member == "" {
					continue
				}
				if looksLikeWWN(member) {
					z.Members = append(z.Members, &ir.ZoneMember{
						Type:  "pwwn",
						Value: normalizeWWN(member),
					})
				} else {
					z.Members = append(z.Members, &ir.ZoneMember{
						Type:  "alias",
						Value: member,
					})
				}
			}
			cfg.Zones[name] = z
			continue
		}

		if m := reCfgCreate.FindStringSubmatch(trimmed); m != nil {
			name := m[1]
			zoneList := m[2]
			var zoneNames []string
			for _, raw := range strings.Split(zoneList, ";") {
				zoneName := strings.TrimSpace(raw)
				if zoneName != "" {
					zoneNames = append(zoneNames, zoneName)
				}
			}
			cfg.ZoneConfigs[name] = &ir.ZoneConfig{Name: name, VSAN: 0, ZoneNames: zoneNames}
			continue
		}
		// Unrecognized lines are silently skipped (warn-and-continue pattern)
	}
}

// parseMemberLine parses a cfgshow member data line. It strips backslash
// continuation markers and returns the list of member tokens and whether the
// next line continues this member group.
func parseMemberLine(line string) (members []string, continues bool) {
	trimmed := strings.TrimSpace(line)
	if strings.HasSuffix(trimmed, `\`) {
		continues = true
		trimmed = strings.TrimSuffix(trimmed, `\`)
		trimmed = strings.TrimSpace(trimmed)
	}
	for _, part := range strings.Split(trimmed, ";") {
		member := strings.TrimSpace(part)
		if member != "" {
			members = append(members, member)
		}
	}
	return members, continues
}

// looksLikeWWN returns true when s looks like a pWWN (contains a colon).
// Brocade alias names cannot contain colons per FOS character set rules,
// so this heuristic reliably distinguishes alias references from raw pWWNs.
func looksLikeWWN(s string) bool {
	return strings.Contains(s, ":")
}

// normalizeWWN converts a WWN string to normalized lowercase colon-hex format.
// If the compact (no-colon) length is not 16 hex chars, returns the input
// lowercased. This is a local copy of the MDS parser helper — per project
// convention, IR package has zero imports so no shared WWN utility exists.
func normalizeWWN(raw string) string {
	compact := strings.ReplaceAll(raw, ":", "")
	compact = strings.ToLower(compact)
	if len(compact) != 16 {
		return strings.ToLower(raw)
	}
	parts := make([]string, 8)
	for i := range 8 {
		parts[i] = compact[i*2 : i*2+2]
	}
	return strings.Join(parts, ":")
}
