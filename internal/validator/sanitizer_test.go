package validator

import (
	"strings"
	"testing"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/stretchr/testify/require"
)

// makeAlias constructs an *ir.Alias with the given name and a placeholder PWWN.
func makeAlias(name string) *ir.Alias {
	return &ir.Alias{Name: name, PWWN: "10:00:00:00:c9:00:00:01"}
}

// makeZone constructs an *ir.Zone with the given name and VSAN 0.
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

// makeCfg builds a minimal *ir.ZoningConfig with a Brocade source format.
func makeCfg() *ir.ZoningConfig {
	return &ir.ZoningConfig{
		Aliases:      make(map[string]*ir.Alias),
		Zones:        make(map[string]*ir.Zone),
		ZoneConfigs:  make(map[string]*ir.ZoneConfig),
		SourceFormat: "brocade-fos",
		Warnings:     []string{},
	}
}

// makeMDSCfg builds a minimal *ir.ZoningConfig with an MDS source format.
func makeMDSCfg() *ir.ZoningConfig {
	return &ir.ZoningConfig{
		Aliases:      make(map[string]*ir.Alias),
		Zones:        make(map[string]*ir.Zone),
		ZoneConfigs:  make(map[string]*ir.ZoneConfig),
		SourceFormat: "mds-nxos",
		Warnings:     []string{},
	}
}

func TestSanitize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fosVersion string
		input      *ir.ZoningConfig
		checkFn    func(t *testing.T, out *ir.ZoningConfig)
	}{
		// ─── SANI-01: Truncation ─────────────────────────────────────────────────

		{
			name:       "alias name exceeding 63 chars is truncated with warning",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				longName := strings.Repeat("A", 70)
				cfg.Aliases[longName] = makeAlias(longName)
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				expected := strings.Repeat("A", 63)
				require.Contains(t, out.Aliases, expected, "alias with truncated key must exist")
				require.Equal(t, expected, out.Aliases[expected].Name, "alias .Name must be truncated to 63 chars")
				require.GreaterOrEqual(t, len(out.Warnings), 1, "at least one warning must be emitted")
				found := false
				for _, w := range out.Warnings {
					if strings.Contains(w, "truncated") {
						found = true
						break
					}
				}
				require.True(t, found, "a warning containing 'truncated' must be present")
			},
		},
		{
			name:       "zone name exceeding 63 chars is truncated with warning",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				longName := strings.Repeat("Z", 70)
				cfg.Zones[longName] = makeZone(longName)
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				expected := strings.Repeat("Z", 63)
				require.Contains(t, out.Zones, expected, "zone with truncated key must exist")
				require.Equal(t, expected, out.Zones[expected].Name, "zone .Name must be 63 chars")
				found := false
				for _, w := range out.Warnings {
					if strings.Contains(w, "truncated") {
						found = true
						break
					}
				}
				require.True(t, found, "a warning containing 'truncated' must be present")
			},
		},
		{
			name:       "name exactly 63 chars is not truncated",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				exactName := strings.Repeat("B", 63)
				cfg.Aliases[exactName] = makeAlias(exactName)
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				expected := strings.Repeat("B", 63)
				require.Contains(t, out.Aliases, expected, "alias must still be present with unchanged key")
				require.Empty(t, out.Warnings, "no warnings expected for exactly-63-char name")
			},
		},

		// ─── SANI-02: Character Replacement ──────────────────────────────────────

		{
			name:       "hyphen replaced with underscore in pre-8.1 mode",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["Server-HBA-A"] = makeAlias("Server-HBA-A")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, out.Aliases, "Server_HBA_A", "alias key must have hyphens replaced")
				require.Equal(t, "Server_HBA_A", out.Aliases["Server_HBA_A"].Name, "alias .Name must have hyphens replaced")
				found := false
				for _, w := range out.Warnings {
					if strings.Contains(w, "Server-HBA-A") || strings.Contains(w, "Server_HBA_A") {
						found = true
						break
					}
				}
				require.True(t, found, "a warning mentioning the alias name must be present")
			},
		},
		{
			name:       "dollar and caret replaced in pre-8.1 mode",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["Host$Port^1"] = makeAlias("Host$Port^1")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, out.Aliases, "Host_Port_1", "alias key must have $ and ^ replaced")
				require.GreaterOrEqual(t, len(out.Warnings), 1, "at least one warning must be emitted")
			},
		},
		{
			name:       "hyphen permitted in 8.1+ mode",
			fosVersion: "8.1+",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["Server-HBA-A"] = makeAlias("Server-HBA-A")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, out.Aliases, "Server-HBA-A", "alias key must be unchanged in 8.1+ mode")
				require.Empty(t, out.Warnings, "no warnings expected for valid 8.1+ name")
			},
		},
		{
			name:       "dollar and caret permitted in 8.1+ mode",
			fosVersion: "8.1+",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["Host$Port^1"] = makeAlias("Host$Port^1")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, out.Aliases, "Host$Port^1", "alias key must be unchanged in 8.1+ mode")
				require.Empty(t, out.Warnings, "no warnings expected for valid 8.1+ name")
			},
		},
		{
			name:       "at-sign replaced in both modes",
			fosVersion: "8.1+",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["Host@Domain"] = makeAlias("Host@Domain")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, out.Aliases, "Host_Domain", "@ must be replaced even in 8.1+ mode")
				require.GreaterOrEqual(t, len(out.Warnings), 1, "a warning must be emitted for @ replacement")
			},
		},

		// ─── SANI-03: Collision Detection ────────────────────────────────────────

		{
			name:       "two aliases colliding after char replacement are disambiguated",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["A-B"] = makeAlias("A-B")
				cfg.Aliases["A_B"] = makeAlias("A_B")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				// Both should sanitize to "A_B"; one gets disambiguated with _2
				require.Len(t, out.Aliases, 2, "both aliases must be preserved after disambiguation")
				require.Contains(t, out.Aliases, "A_B", "first sanitized alias must exist as A_B")
				require.Contains(t, out.Aliases, "A_B_2", "second sanitized alias must exist as A_B_2")
				// The two aliases must have different .Name values
				require.NotEqual(t, out.Aliases["A_B"].Name, out.Aliases["A_B_2"].Name, "disambiguated aliases must have different names")
				found := false
				for _, w := range out.Warnings {
					if strings.Contains(w, "collision") {
						found = true
						break
					}
				}
				require.True(t, found, "a warning containing 'collision' must be present")
			},
		},
		{
			name:       "two zones colliding after truncation are disambiguated",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				// Both names share the first 63 chars but differ at char 64
				base := strings.Repeat("Z", 63)
				name1 := base + "X"
				name2 := base + "Y"
				cfg.Zones[name1] = makeZone(name1)
				cfg.Zones[name2] = makeZone(name2)
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, out.Zones, 2, "both zones must be preserved after disambiguation")
				truncated := strings.Repeat("Z", 63)
				require.Contains(t, out.Zones, truncated, "first truncated zone must exist")
				// Second one should have a disambiguating suffix
				found := false
				for k := range out.Zones {
					if k != truncated && strings.HasPrefix(k, strings.Repeat("Z", 60)) {
						found = true
						break
					}
				}
				require.True(t, found, "second disambiguated zone must exist with a suffix")
				collisionWarning := false
				for _, w := range out.Warnings {
					if strings.Contains(w, "collision") {
						collisionWarning = true
						break
					}
				}
				require.True(t, collisionWarning, "a collision warning must be emitted")
			},
		},
		{
			name:       "collision suffix does not exceed 63 chars",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				// Two names that both truncate to the same 63-char string
				base := strings.Repeat("C", 63)
				name1 := base + "1"
				name2 := base + "2"
				cfg.Aliases[name1] = makeAlias(name1)
				cfg.Aliases[name2] = makeAlias(name2)
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				for k := range out.Aliases {
					require.LessOrEqual(t, len(k), 63, "all alias names must be <= 63 chars after disambiguation")
				}
			},
		},

		// ─── Cross-reference Updates ──────────────────────────────────────────────

		{
			name:       "zone member alias value updated when alias renamed",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["A-host"] = makeAlias("A-host")
				cfg.Zones["MyZone"] = makeZone("MyZone",
					&ir.ZoneMember{Type: "alias", Value: "A-host"},
				)
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				zone, ok := out.Zones["MyZone"]
				require.True(t, ok, "zone 'MyZone' must exist")
				require.Len(t, zone.Members, 1, "zone must have 1 member")
				require.Equal(t, "alias", zone.Members[0].Type, "member type must be 'alias'")
				require.Equal(t, "A_host", zone.Members[0].Value, "zone member value must be updated to sanitized alias name")
			},
		},
		{
			name:       "zoneset zone name updated when zone renamed",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Zones["My-Zone"] = makeZone("My-Zone")
				cfg.ZoneConfigs["MyCfg"] = makeZoneConfig("MyCfg", "My-Zone")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				zoneset, ok := out.ZoneConfigs["MyCfg"]
				require.True(t, ok, "zoneset 'MyCfg' must exist")
				require.Len(t, zoneset.ZoneNames, 1, "zoneset must have 1 zone reference")
				require.Equal(t, "My_Zone", zoneset.ZoneNames[0], "zoneset zone name must be updated to sanitized zone name")
			},
		},

		// ─── MDS Composite Key Handling ──────────────────────────────────────────

		{
			name:       "MDS zone composite key reconstructed after rename",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeMDSCfg()
				// MDS zones use composite key "name@vsanN"
				cfg.Zones["My-Zone@vsan10"] = makeZoneVSAN("My-Zone", 10)
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Contains(t, out.Zones, "My_Zone@vsan10", "composite key must be reconstructed with sanitized name")
				require.Equal(t, "My_Zone", out.Zones["My_Zone@vsan10"].Name, "zone .Name must be sanitized")
				require.NotContains(t, out.Zones, "My-Zone@vsan10", "old composite key must not remain")
			},
		},

		// ─── No-op Case ───────────────────────────────────────────────────────────

		{
			name:       "clean names produce no warnings",
			fosVersion: "pre-8.1",
			input: func() *ir.ZoningConfig {
				cfg := makeCfg()
				cfg.Aliases["CleanName_1"] = makeAlias("CleanName_1")
				cfg.Zones["CleanZone_2"] = makeZone("CleanZone_2")
				cfg.ZoneConfigs["CleanCfg_3"] = makeZoneConfig("CleanCfg_3", "CleanZone_2")
				return cfg
			}(),
			checkFn: func(t *testing.T, out *ir.ZoningConfig) {
				t.Helper()
				require.Empty(t, out.Warnings, "no warnings expected for already-clean names")
				require.Contains(t, out.Aliases, "CleanName_1", "clean alias must be unchanged")
				require.Contains(t, out.Zones, "CleanZone_2", "clean zone must be unchanged")
				require.Contains(t, out.ZoneConfigs, "CleanCfg_3", "clean zoneset must be unchanged")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := Sanitize(tt.input, tt.fosVersion)
			tt.checkFn(t, out)
		})
	}
}
