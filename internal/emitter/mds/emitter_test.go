package mds

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/stretchr/testify/require"
)

// makeAlias constructs an *ir.Alias with the given name and pWWN (VSAN 0 = fabric-wide).
func makeAlias(name, pwwn string) *ir.Alias {
	return &ir.Alias{Name: name, PWWN: pwwn, VSAN: 0}
}

// makeZone constructs an *ir.Zone with the given name (VSAN 0 as sentinel).
func makeZone(name string, members ...*ir.ZoneMember) *ir.Zone {
	return &ir.Zone{Name: name, VSAN: 0, Members: members}
}

// makeZoneVSAN constructs an *ir.Zone with the given name and explicit VSAN.
func makeZoneVSAN(name string, vsan int, members ...*ir.ZoneMember) *ir.Zone {
	return &ir.Zone{Name: name, VSAN: vsan, Members: members}
}

// makeZoneConfig constructs an *ir.ZoneConfig with the given name and zone names (VSAN 0 sentinel).
func makeZoneConfig(name string, zoneNames ...string) *ir.ZoneConfig {
	return &ir.ZoneConfig{Name: name, VSAN: 0, ZoneNames: zoneNames}
}

// makeZoneConfigVSAN constructs an *ir.ZoneConfig with the given name, explicit VSAN, and zone names.
func makeZoneConfigVSAN(name string, vsan int, zoneNames ...string) *ir.ZoneConfig {
	return &ir.ZoneConfig{Name: name, VSAN: vsan, ZoneNames: zoneNames}
}

// makeMember constructs an *ir.ZoneMember with the given type and value.
func makeMember(typ, val string) *ir.ZoneMember {
	return &ir.ZoneMember{Type: typ, Value: val}
}

// makeCfg builds a minimal *ir.ZoningConfig with initialized maps and Brocade source format.
func makeCfg() *ir.ZoningConfig {
	return &ir.ZoningConfig{
		Aliases:      make(map[string]*ir.Alias),
		Zones:        make(map[string]*ir.Zone),
		ZoneConfigs:  make(map[string]*ir.ZoneConfig),
		SourceFormat: "brocade-fos",
		Warnings:     []string{},
	}
}

func TestEmit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   *ir.ZoningConfig
		checkFn func(t *testing.T, output string, cfg *ir.ZoningConfig)
	}{
		// Test 1: CONV-04 — device-alias database block emitted for every alias
		{
			name: "device-alias database block emitted for every alias (CONV-04)",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["host_01"] = makeAlias("host_01", "10:00:00:00:c9:ab:cd:ef")
				cfg.Aliases["storage_01"] = makeAlias("storage_01", "50:05:07:61:01:23:45:67")
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, "device-alias database")
				require.Contains(t, output, "  device-alias name host_01 pwwn 10:00:00:00:c9:ab:cd:ef")
				require.Contains(t, output, "  device-alias name storage_01 pwwn 50:05:07:61:01:23:45:67")
				require.Contains(t, output, "device-alias commit")
			},
		},

		// Test 2: CONV-05 — zone block with alias members
		{
			name: "zone block emitted with device-alias members (CONV-05)",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["host_01"] = makeAlias("host_01", "10:00:00:00:c9:ab:cd:ef")
				cfg.Zones["fabric_zone1"] = makeZone("fabric_zone1",
					makeMember("alias", "host_01"),
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, "zone name fabric_zone1 vsan 1")
				require.Contains(t, output, "  member device-alias host_01")
			},
		},

		// Test 3: CONV-05 — zone block with pWWN members
		{
			name: "zone block emitted with pwwn members (CONV-05)",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Zones["direct_zone"] = makeZone("direct_zone",
					makeMember("pwwn", "50:05:07:61:01:23:45:67"),
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, "zone name direct_zone vsan 1")
				require.Contains(t, output, "  member pwwn 50:05:07:61:01:23:45:67")
			},
		},

		// Test 4: CONV-06 — zoneset block plus zoneset activate emitted (activate is NOT commented)
		{
			name: "zoneset block and non-commented zoneset activate emitted (CONV-06)",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Zones["fabric_zone1"] = makeZone("fabric_zone1",
					makeMember("pwwn", "10:00:00:00:c9:ab:cd:ef"),
				)
				cfg.Zones["fabric_zone2"] = makeZone("fabric_zone2",
					makeMember("pwwn", "50:05:07:61:01:23:45:67"),
				)
				cfg.ZoneConfigs["Production_cfg"] = makeZoneConfig("Production_cfg",
					"fabric_zone1", "fabric_zone2",
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, "zoneset name Production_cfg vsan 1")
				require.Contains(t, output, "  member fabric_zone1")
				require.Contains(t, output, "  member fabric_zone2")
				require.Contains(t, output, "zoneset activate name Production_cfg vsan 1")
				// Ensure activate is NOT commented out (unlike Brocade cfgenable)
				for _, line := range strings.Split(output, "\n") {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "#") && strings.Contains(trimmed, "zoneset activate") {
						t.Errorf("zoneset activate must NOT be commented out, got: %q", line)
					}
				}
			},
		},

		// Test 5: OUT-03 — canonical output order: device-alias block then zones then zonesets
		{
			name: "canonical output order: device-alias then zone then zoneset (OUT-03)",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["host_01"] = makeAlias("host_01", "10:00:00:00:c9:ab:cd:ef")
				cfg.Zones["zone_01"] = makeZone("zone_01",
					makeMember("alias", "host_01"),
				)
				cfg.ZoneConfigs["cfg_01"] = makeZoneConfig("cfg_01", "zone_01")
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				aliasPos := strings.Index(output, "device-alias database")
				zonePos := strings.Index(output, "zone name")
				zonesetPos := strings.Index(output, "zoneset name")
				require.Greater(t, aliasPos, -1, "device-alias database must be present")
				require.Greater(t, zonePos, -1, "zone name must be present")
				require.Greater(t, zonesetPos, -1, "zoneset name must be present")
				require.Less(t, aliasPos, zonePos, "device-alias block must appear before zone blocks")
				require.Less(t, zonePos, zonesetPos, "zone blocks must appear before zoneset block")
			},
		},

		// Test 6: VSAN 0 sentinel — Brocade-sourced IR with VSAN 0 must never emit "vsan 0"
		{
			name: "VSAN 0 sentinel resolved to vsan 1 — vsan 0 never appears in output",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				// All structs have VSAN: 0 (Brocade sentinel)
				cfg.Aliases["host_01"] = &ir.Alias{Name: "host_01", PWWN: "10:00:00:00:c9:ab:cd:ef", VSAN: 0}
				cfg.Zones["zone_01"] = &ir.Zone{
					Name: "zone_01", VSAN: 0,
					Members: []*ir.ZoneMember{
						{Type: "alias", Value: "host_01"},
					},
				}
				cfg.ZoneConfigs["cfg_01"] = &ir.ZoneConfig{
					Name: "cfg_01", VSAN: 0, ZoneNames: []string{"zone_01"},
				}
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.NotContains(t, output, "vsan 0",
					"VSAN 0 sentinel must never appear literally in emitted output")
				require.Contains(t, output, "vsan 1",
					"resolved VSAN 1 must appear in emitted output")
			},
		},

		// Test 7: empty zone (all members unsupported) is skipped with warning; excluded from zoneset
		{
			name: "empty zone (all members unsupported) skipped with warning; excluded from zoneset",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Zones["bad_zone"] = makeZone("bad_zone",
					makeMember("unsupported", "interface fc1/1"),
				)
				cfg.ZoneConfigs["cfg_01"] = makeZoneConfig("cfg_01", "bad_zone")
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				// bad_zone must NOT appear as a zone definition
				require.NotContains(t, output, "zone name bad_zone",
					"skipped zone must not produce a zone block")

				// cfg.Warnings must contain a warning mentioning bad_zone and skipped
				found := false
				for _, w := range cfg.Warnings {
					if strings.Contains(w, "bad_zone") && strings.Contains(w, "skipped") {
						found = true
						break
					}
				}
				require.True(t, found, "expected warning about bad_zone being skipped, warnings: %v", cfg.Warnings)

				// If a zoneset line is present, it must NOT include bad_zone as a member
				if strings.Contains(output, "zoneset name") {
					for _, line := range strings.Split(output, "\n") {
						trimmed := strings.TrimSpace(line)
						if trimmed == "member bad_zone" {
							t.Errorf("zoneset must not reference the skipped zone bad_zone, got: %q", line)
						}
					}
				}
			},
		},

		// Test 8: multi-VSAN passthrough — MDS-sourced IR with composite map keys emits correct zone names
		{
			name: "multi-VSAN MDS IR passthrough: zone.Name used not map key; @vsan never appears (OUT-03)",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.SourceFormat = "mds-nxos"
				// MDS parser uses composite keys "name@vsanN" with Zone.Name = plain name
				cfg.Zones["zoneA@vsan10"] = makeZoneVSAN("zoneA", 10,
					makeMember("pwwn", "10:00:00:00:c9:ab:cd:ef"),
				)
				cfg.Zones["zoneB@vsan20"] = makeZoneVSAN("zoneB", 20,
					makeMember("pwwn", "50:05:07:61:01:23:45:67"),
				)
				cfg.ZoneConfigs["cfg_vsan10"] = makeZoneConfigVSAN("cfg_vsan10", 10, "zoneA")
				cfg.ZoneConfigs["cfg_vsan20"] = makeZoneConfigVSAN("cfg_vsan20", 20, "zoneB")
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				// Zone blocks must use zone.Name (plain name), not the composite map key
				require.Contains(t, output, "zone name zoneA vsan 10",
					"zone from VSAN 10 must use plain name and actual VSAN")
				require.Contains(t, output, "zone name zoneB vsan 20",
					"zone from VSAN 20 must use plain name and actual VSAN")
				// Composite key suffix must not leak into output
				require.NotContains(t, output, "@vsan",
					"composite map key suffix must not appear in emitted output")
			},
		},

		// Test 9: mixed members — alias and pwwn emitted; unsupported member silently skipped
		{
			name: "mixed zone members: alias and pwwn emitted; unsupported silently skipped",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Zones["mixed_zone"] = makeZone("mixed_zone",
					makeMember("alias", "host_01"),
					makeMember("pwwn", "50:05:07:61:01:23:45:67"),
					makeMember("unsupported", "interface fc1/1"),
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, "  member device-alias host_01",
					"alias member must be emitted as member device-alias")
				require.Contains(t, output, "  member pwwn 50:05:07:61:01:23:45:67",
					"pwwn member must be emitted as member pwwn")
				require.NotContains(t, output, "interface fc1/1",
					"unsupported member must not appear in emitted output")
			},
		},

		// Test 10: deterministic output — two consecutive Emit calls produce byte-identical output
		{
			name: "deterministic output: repeated Emit calls produce identical output",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				// Deliberately unsorted alias names to stress map ordering
				cfg.Aliases["eee"] = makeAlias("eee", "10:00:00:00:c9:ee:ee:ee")
				cfg.Aliases["aaa"] = makeAlias("aaa", "10:00:00:00:c9:aa:aa:aa")
				cfg.Aliases["ddd"] = makeAlias("ddd", "10:00:00:00:c9:dd:dd:dd")
				cfg.Aliases["bbb"] = makeAlias("bbb", "10:00:00:00:c9:bb:bb:bb")
				cfg.Aliases["ccc"] = makeAlias("ccc", "10:00:00:00:c9:cc:cc:cc")
				// Deliberately unsorted zone names
				cfg.Zones["zone_c"] = makeZone("zone_c",
					makeMember("alias", "ccc"),
					makeMember("alias", "eee"),
				)
				cfg.Zones["zone_a"] = makeZone("zone_a",
					makeMember("alias", "aaa"),
					makeMember("alias", "bbb"),
				)
				cfg.Zones["zone_b"] = makeZone("zone_b",
					makeMember("alias", "ddd"),
				)
				cfg.ZoneConfigs["test_cfg"] = makeZoneConfig("test_cfg",
					"zone_c", "zone_a", "zone_b",
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				// Run Emit a second time on the same IR and compare outputs
				var buf2 bytes.Buffer
				err2 := Emit(cfg, &buf2)
				require.NoError(t, err2)
				require.Equal(t, output, buf2.String(),
					"repeated Emit calls must produce identical output (deterministic ordering)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			err := Emit(tt.input, &buf)
			require.NoError(t, err)
			tt.checkFn(t, buf.String(), tt.input)
		})
	}
}
