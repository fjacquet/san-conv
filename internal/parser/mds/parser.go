package mds

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/fjacquet/san-conv/internal/ir"
)

// Compiled regexes for all MDS NX-OS running-config constructs.
var (
	reDeviceAliasDBHeader = regexp.MustCompile(`^device-alias\s+database\s*$`)
	reDeviceAliasEntry    = regexp.MustCompile(`^\s+device-alias\s+name\s+(\S+)\s+pwwn\s+(\S+)`)
	reDeviceAliasCommit   = regexp.MustCompile(`^device-alias\s+commit\s*$`)
	reFcAliasHeader       = regexp.MustCompile(`^fcalias\s+name\s+(\S+)\s+vsan\s+(\d+)`)
	reFcAliasMember       = regexp.MustCompile(`^\s+member\s+pwwn\s+(\S+)`)
	reZoneHeader          = regexp.MustCompile(`^zone\s+name\s+(\S+)\s+vsan\s+(\d+)`)
	reZonesetHeader       = regexp.MustCompile(`^zoneset\s+name\s+(\S+)\s+vsan\s+(\d+)`)
	reZonesetActivate     = regexp.MustCompile(`^zoneset\s+activate\s+name\s+(\S+)\s+vsan\s+(\d+)`)
	reMemberDeviceAlias   = regexp.MustCompile(`^\s+member\s+device-alias\s+(\S+)`)
	reMemberFcAlias       = regexp.MustCompile(`^\s+member\s+fcalias\s+(\S+)`)
	reMemberPWWN          = regexp.MustCompile(`^\s+member\s+pwwn\s+(\S+)`)
	reMemberPWWNRole      = regexp.MustCompile(`^\s+member\s+pwwn\s+\S+\s+(init|target|both)\s*$`)
	reMemberInterface     = regexp.MustCompile(`^\s+member\s+interface\s+`)
	reMemberFcid          = regexp.MustCompile(`^\s+member\s+fcid\s+`)
	reMemberIPAddr        = regexp.MustCompile(`^\s+member\s+ip-address\s+`)
	reMemberSymbolicNode  = regexp.MustCompile(`^\s+member\s+symbolic-nodename\s+`)
	reMemberFwwn          = regexp.MustCompile(`^\s+member\s+fwwn\s+`)
	reIVRZoneHeader       = regexp.MustCompile(`^ivr\s+zone\s+name\s+`)
	reIVRZonesetHeader    = regexp.MustCompile(`^ivr\s+zoneset\s+name\s+`)
	reZonesetMember       = regexp.MustCompile(`^\s+member\s+(\S+)`)
)

// Parse reads a Cisco MDS NX-OS running-config from r and returns a populated
// *ir.ZoningConfig. Non-fatal issues are appended to cfg.Warnings.
// Parse only returns an error for I/O failures.
func Parse(r io.Reader) (*ir.ZoningConfig, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	cfg := &ir.ZoningConfig{
		SourceFormat: "mds-nxos",
		Aliases:      make(map[string]*ir.Alias),
		Zones:        make(map[string]*ir.Zone),
		ZoneConfigs:  make(map[string]*ir.ZoneConfig),
	}

	pass1BuildAliases(lines, cfg)
	pass2BuildZones(lines, cfg)

	return cfg, nil
}

// pass1BuildAliases processes device-alias database and fcalias blocks to
// populate cfg.Aliases.
func pass1BuildAliases(lines []string, cfg *ir.ZoningConfig) {
	const (
		stateIdle        = iota
		stateDeviceAliasDB
		stateFcAlias
	)

	state := stateIdle
	var currentFcAlias *ir.Alias

	for _, line := range lines {
		// CRITICAL ORDER: check commit BEFORE entry to avoid mis-parsing
		if reDeviceAliasCommit.MatchString(line) {
			state = stateIdle
			currentFcAlias = nil
			continue
		}

		if reDeviceAliasDBHeader.MatchString(line) {
			state = stateDeviceAliasDB
			continue
		}

		if m := reFcAliasHeader.FindStringSubmatch(line); m != nil {
			name := m[1]
			var vsan int
			fmt.Sscanf(m[2], "%d", &vsan)
			a := &ir.Alias{Name: name, VSAN: vsan}
			cfg.Aliases[name] = a
			currentFcAlias = a
			state = stateFcAlias
			continue
		}

		// If we see a top-level keyword while in a block state, exit block
		if state != stateIdle && isTopLevelKeyword(line) {
			state = stateIdle
			currentFcAlias = nil
			continue
		}

		switch state {
		case stateDeviceAliasDB:
			if m := reDeviceAliasEntry.FindStringSubmatch(line); m != nil {
				name := m[1]
				pwwn := normalizeWWN(m[2])
				cfg.Aliases[name] = &ir.Alias{Name: name, PWWN: pwwn, VSAN: 0}
			}
		case stateFcAlias:
			if currentFcAlias != nil {
				if m := reFcAliasMember.FindStringSubmatch(line); m != nil {
					// Only set PWWN once (first member only)
					if currentFcAlias.PWWN == "" {
						currentFcAlias.PWWN = normalizeWWN(m[1])
					}
				}
			}
		}
	}
}

// pass2BuildZones processes zone and zoneset blocks to populate cfg.Zones and
// cfg.ZoneConfigs.
func pass2BuildZones(lines []string, cfg *ir.ZoningConfig) {
	const (
		stateIdle     = iota
		stateZone
		stateZoneset
		stateIVRSkip
	)

	state := stateIdle
	var currentZone *ir.Zone
	var currentZoneset *ir.ZoneConfig
	seenVSANs := make(map[int]bool)

	for _, line := range lines {
		// CRITICAL ORDER: IVR must come before zone checks (IVR lines contain "zone name")
		if reIVRZoneHeader.MatchString(line) {
			// Extract name for warning message
			parts := strings.Fields(line)
			ivrName := ""
			if len(parts) >= 4 {
				ivrName = parts[3]
			}
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("IVR zone %q skipped — no FOS equivalent", ivrName))
			state = stateIVRSkip
			currentZone = nil
			currentZoneset = nil
			continue
		}

		if reIVRZonesetHeader.MatchString(line) {
			parts := strings.Fields(line)
			ivrName := ""
			if len(parts) >= 4 {
				ivrName = parts[3]
			}
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf("IVR zoneset %q skipped — no FOS equivalent", ivrName))
			state = stateIVRSkip
			currentZone = nil
			currentZoneset = nil
			continue
		}

		if m := reZoneHeader.FindStringSubmatch(line); m != nil {
			name := m[1]
			var vsan int
			fmt.Sscanf(m[2], "%d", &vsan)

			// Track VSANs for multi-VSAN warning
			if !seenVSANs[vsan] {
				seenVSANs[vsan] = true
				if len(seenVSANs) == 2 {
					cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
						"multi-VSAN config detected (%d VSANs) — zones are VSAN-scoped; all converted to single Brocade fabric",
						len(seenVSANs),
					))
				}
			}

			key := fmt.Sprintf("%s@vsan%d", name, vsan)
			z := &ir.Zone{Name: name, VSAN: vsan}
			cfg.Zones[key] = z
			currentZone = z
			currentZoneset = nil
			state = stateZone
			continue
		}

		// zoneset activate — silently ignore (no Active field in IR)
		if reZonesetActivate.MatchString(line) {
			state = stateIdle
			currentZone = nil
			currentZoneset = nil
			continue
		}

		if m := reZonesetHeader.FindStringSubmatch(line); m != nil {
			name := m[1]
			var vsan int
			fmt.Sscanf(m[2], "%d", &vsan)

			// Track VSANs for multi-VSAN warning
			if !seenVSANs[vsan] {
				seenVSANs[vsan] = true
				if len(seenVSANs) == 2 {
					cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
						"multi-VSAN config detected (%d VSANs) — zones are VSAN-scoped; all converted to single Brocade fabric",
						len(seenVSANs),
					))
				}
			}

			key := fmt.Sprintf("%s@vsan%d", name, vsan)
			zc := &ir.ZoneConfig{Name: name, VSAN: vsan}
			cfg.ZoneConfigs[key] = zc
			currentZoneset = zc
			currentZone = nil
			state = stateZoneset
			continue
		}

		// Top-level keyword transitions to idle (except blank/comment lines)
		if state != stateIdle && isTopLevelKeyword(line) {
			state = stateIdle
			currentZone = nil
			currentZoneset = nil
			continue
		}

		// Process members based on current state
		switch state {
		case stateZone:
			if currentZone == nil {
				continue
			}
			processZoneMember(line, currentZone, cfg)
		case stateZoneset:
			if currentZoneset == nil {
				continue
			}
			if m := reZonesetMember.FindStringSubmatch(line); m != nil {
				currentZoneset.ZoneNames = append(currentZoneset.ZoneNames, m[1])
			}
		}
	}
}

// processZoneMember classifies and handles a single zone member line.
// Unsupported member types append a warning and are NOT added to zone.Members.
func processZoneMember(line string, zone *ir.Zone, cfg *ir.ZoningConfig) {
	// Check unsupported types FIRST (before pwwn, alias checks)
	if reMemberInterface.MatchString(line) {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q: unsupported member type %q (%s) skipped",
			zone.Name, "interface", strings.TrimSpace(line),
		))
		return
	}
	if reMemberFcid.MatchString(line) {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q: unsupported member type %q (%s) skipped",
			zone.Name, "fcid", strings.TrimSpace(line),
		))
		return
	}
	if reMemberIPAddr.MatchString(line) {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q: unsupported member type %q (%s) skipped",
			zone.Name, "ip-address", strings.TrimSpace(line),
		))
		return
	}
	if reMemberSymbolicNode.MatchString(line) {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q: unsupported member type %q (%s) skipped",
			zone.Name, "symbolic-nodename", strings.TrimSpace(line),
		))
		return
	}
	if reMemberFwwn.MatchString(line) {
		cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
			"zone %q: unsupported member type %q (%s) skipped",
			zone.Name, "fwwn", strings.TrimSpace(line),
		))
		return
	}

	// device-alias member
	if m := reMemberDeviceAlias.FindStringSubmatch(line); m != nil {
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "alias", Value: m[1]})
		return
	}

	// fcalias member
	if m := reMemberFcAlias.FindStringSubmatch(line); m != nil {
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "alias", Value: m[1]})
		return
	}

	// pWWN member — check for smart-zoning role suffix AFTER extracting WWN
	if m := reMemberPWWN.FindStringSubmatch(line); m != nil {
		wwn := normalizeWWN(m[1])
		zone.Members = append(zone.Members, &ir.ZoneMember{Type: "pwwn", Value: wwn})

		// Check for smart-zoning role keyword
		if roleMatch := reMemberPWWNRole.FindStringSubmatch(line); roleMatch != nil {
			role := roleMatch[1]
			cfg.Warnings = append(cfg.Warnings, fmt.Sprintf(
				"zone %q: smart-zoning role %q on member %s stripped — no FOS equivalent",
				zone.Name, role, wwn,
			))
		}
		return
	}
}

// normalizeWWN converts a WWN string to normalized lowercase colon-hex format.
// If the compact (no-colon) length is not 16 hex chars, returns the input lowercased.
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

// isTopLevelKeyword returns true when a line starts a new top-level block.
// Blank lines and '!' comments are NOT top-level keywords (they continue current block).
func isTopLevelKeyword(line string) bool {
	if len(line) == 0 {
		return false
	}
	if line[0] == '!' {
		return false
	}
	// A top-level keyword has no leading whitespace
	return line[0] != ' ' && line[0] != '\t'
}
