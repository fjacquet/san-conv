package brocade

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/stretchr/testify/require"
)

// makeAlias constructs an *ir.Alias with the given name and pWWN.
func makeAlias(name, pwwn string) *ir.Alias {
	return &ir.Alias{Name: name, PWWN: pwwn, VSAN: 0}
}

// makeZone constructs an *ir.Zone with the given name (VSAN 0).
func makeZone(name string, members ...*ir.ZoneMember) *ir.Zone {
	return &ir.Zone{Name: name, VSAN: 0, Members: members}
}

// makeZoneVSAN constructs an *ir.Zone with the given name and explicit VSAN.
func makeZoneVSAN(name string, vsan int, members ...*ir.ZoneMember) *ir.Zone {
	return &ir.Zone{Name: name, VSAN: vsan, Members: members}
}

// makeZoneConfig constructs an *ir.ZoneConfig with the given name and zone names.
func makeZoneConfig(name string, zoneNames ...string) *ir.ZoneConfig {
	return &ir.ZoneConfig{Name: name, VSAN: 0, ZoneNames: zoneNames}
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
		name       string
		input      *ir.ZoningConfig
		scriptMode bool
		checkFn    func(t *testing.T, output string, cfg *ir.ZoningConfig)
	}{
		// Test 1: CONV-01 — alicreate emitted for every alias
		{
			name:       "commands-only mode emits alicreate for every alias (CONV-01)",
			scriptMode: false,
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["host_01"] = makeAlias("host_01", "10:00:00:00:c9:ab:cd:ef")
				cfg.Aliases["storage_01"] = makeAlias("storage_01", "50:05:07:61:01:23:45:67")
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, `alicreate "host_01", "10:00:00:00:c9:ab:cd:ef"`)
				require.Contains(t, output, `alicreate "storage_01", "50:05:07:61:01:23:45:67"`)
			},
		},

		// Test 2: CONV-02 — zonecreate with alias members
		{
			name:       "commands-only mode emits zonecreate with alias members (CONV-02)",
			scriptMode: false,
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["host_01"] = makeAlias("host_01", "10:00:00:00:c9:ab:cd:ef")
				cfg.Aliases["storage_01"] = makeAlias("storage_01", "50:05:07:61:01:23:45:67")
				cfg.Zones["fabric_zone1"] = makeZone("fabric_zone1",
					makeMember("alias", "host_01"),
					makeMember("alias", "storage_01"),
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, `zonecreate "fabric_zone1", "host_01;storage_01"`)
			},
		},

		// Test 3: CONV-02 — zonecreate with pWWN members
		{
			name:       "commands-only mode emits zonecreate with pWWN members (CONV-02)",
			scriptMode: false,
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Zones["direct_zone"] = makeZone("direct_zone",
					makeMember("pwwn", "10:00:00:00:c9:ab:cd:ef"),
					makeMember("pwwn", "50:05:07:61:01:23:45:67"),
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, output, `zonecreate "direct_zone", "10:00:00:00:c9:ab:cd:ef;50:05:07:61:01:23:45:67"`)
			},
		},

		// Test 4: CONV-03 — cfgcreate for every zone config
		{
			name:       "commands-only mode emits cfgcreate for every zone config (CONV-03)",
			scriptMode: false,
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
				require.Contains(t, output, `cfgcreate "Production_cfg", "fabric_zone1;fabric_zone2"`)
			},
		},

		// Test 5: OUT-01 — correct order: alicreate → zonecreate → cfgcreate
		{
			name:       "commands-only mode: correct order alicreate then zonecreate then cfgcreate (OUT-01)",
			scriptMode: false,
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
				aliPos := strings.Index(output, "alicreate")
				zonePos := strings.Index(output, "zonecreate")
				cfgPos := strings.Index(output, "cfgcreate")
				require.Greater(t, aliPos, -1, "alicreate must be present in output")
				require.Greater(t, zonePos, -1, "zonecreate must be present in output")
				require.Greater(t, cfgPos, -1, "cfgcreate must be present in output")
				require.Less(t, aliPos, zonePos, "alicreate must appear before zonecreate")
				require.Less(t, zonePos, cfgPos, "zonecreate must appear before cfgcreate")
			},
		},

		// Test 6: OUT-02 — script mode preamble (defzone --noaccess)
		{
			name:       "script mode: defzone --noaccess preamble present (OUT-02)",
			scriptMode: true,
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
				require.True(t, strings.HasPrefix(output, "defzone --noaccess\n"),
					"output must start with 'defzone --noaccess\\n' preamble")
			},
		},

		// Test 7: OUT-02 — script mode postamble (cfgsave) and cfgenable commented
		{
			name:       "script mode: cfgsave postamble present and cfgenable commented (OUT-02)",
			scriptMode: true,
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Zones["zone_01"] = makeZone("zone_01",
					makeMember("pwwn", "10:00:00:00:c9:ab:cd:ef"),
				)
				cfg.ZoneConfigs["Production_cfg"] = makeZoneConfig("Production_cfg", "zone_01")
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				// cfgenable must appear commented out
				require.Contains(t, output, `# cfgenable "Production_cfg"`,
					"cfgenable must be commented out with '#' prefix")

				// cfgsave must be present
				require.Contains(t, output, "cfgsave",
					"cfgsave postamble must be present in script mode")

				// cfgenable must NOT appear as a bare executable line
				for _, line := range strings.Split(output, "\n") {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "cfgenable") {
						t.Errorf("found bare cfgenable line (without # prefix): %q", line)
					}
				}

				// cfgenable comment must appear before cfgsave
				cfgEnablePos := strings.Index(output, "cfgenable")
				cfgsavePos := strings.Index(output, "cfgsave")
				require.Greater(t, cfgEnablePos, -1, "cfgenable comment must be present")
				require.Greater(t, cfgsavePos, -1, "cfgsave must be present")
				require.Less(t, cfgEnablePos, cfgsavePos,
					"cfgenable comment must appear before cfgsave")
			},
		},

		// Test 8: empty zone (all members unsupported) is skipped with warning
		{
			name:       "empty zone (all members unsupported) is skipped with warning",
			scriptMode: false,
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
				// bad_zone must NOT appear as a zonecreate command
				require.NotContains(t, output, `zonecreate "bad_zone"`,
					"skipped zone must not produce a zonecreate command")

				// cfg.Warnings must contain a warning mentioning bad_zone and skipped
				found := false
				for _, w := range cfg.Warnings {
					if strings.Contains(w, "bad_zone") && strings.Contains(w, "skipped") {
						found = true
						break
					}
				}
				require.True(t, found, "expected warning about bad_zone being skipped")

				// If a cfgcreate line is present, it must NOT contain bad_zone
				if strings.Contains(output, "cfgcreate") {
					// Find the cfgcreate line and verify bad_zone is not in it
					for _, line := range strings.Split(output, "\n") {
						if strings.HasPrefix(line, "cfgcreate") {
							require.NotContains(t, line, "bad_zone",
								"cfgcreate must not reference the skipped zone")
						}
					}
				}
			},
		},

		// Test 9: multi-VSAN MDS IR emits all zones regardless of VSAN
		{
			name:       "multi-VSAN MDS IR emits all zones regardless of VSAN",
			scriptMode: false,
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.SourceFormat = "mds-nxos"
				// MDS IR uses composite keys "name@vsanN" with Zone.Name = plain name
				cfg.Zones["zoneA@vsan10"] = makeZoneVSAN("zoneA", 10,
					makeMember("pwwn", "10:00:00:00:c9:ab:cd:ef"),
				)
				cfg.Zones["zoneB@vsan20"] = makeZoneVSAN("zoneB", 20,
					makeMember("pwwn", "50:05:07:61:01:23:45:67"),
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, output string, cfg *ir.ZoningConfig) {
				t.Helper()
				// Both zones must appear using their .Name field (not composite map key)
				require.Contains(t, output, `zonecreate "zoneA"`,
					"zone from VSAN 10 must be emitted")
				require.Contains(t, output, `zonecreate "zoneB"`,
					"zone from VSAN 20 must be emitted")
				// The VSAN suffix must not appear in the emitted command
				require.NotContains(t, output, "@vsan",
					"VSAN composite key suffix must not appear in emitted output")
			},
		},

		// Test 10: deterministic output — repeated Emit calls produce identical output
		{
			name:       "deterministic output: repeated calls produce identical output",
			scriptMode: false,
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
				err2 := Emit(cfg, &buf2, false)
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
			err := Emit(tt.input, &buf, tt.scriptMode)
			require.NoError(t, err)
			tt.checkFn(t, buf.String(), tt.input)
		})
	}
}
