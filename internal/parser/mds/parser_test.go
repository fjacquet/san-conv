package mds

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fixture string
		checkFn func(t *testing.T, cfg *ir.ZoningConfig)
	}{
		{
			name:    "basic mode with device-alias and fcalias",
			fixture: "basic.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Aliases, 2, "want 2 aliases (device-alias block + fcalias)")
				require.Len(t, cfg.Zones, 1, "want 1 zone")
				require.Len(t, cfg.ZoneConfigs, 1, "want 1 zoneset")
				require.Empty(t, cfg.Warnings, "want 0 warnings")

				zone, ok := cfg.Zones["Server@vsan10"]
				require.True(t, ok, "zone key 'Server@vsan10' must exist")
				require.Len(t, zone.Members, 3, "zone must have 3 members")

				require.Equal(t, "alias", zone.Members[0].Type, "member[0] must be alias")
				require.Equal(t, "alias", zone.Members[1].Type, "member[1] must be alias")
				require.Equal(t, "pwwn", zone.Members[2].Type, "member[2] must be pwwn")
			},
		},
		{
			name:    "enhanced mode device-alias zone members",
			fixture: "enhanced_mode.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Aliases, 2, "want 2 aliases")
				require.Len(t, cfg.Zones, 1, "want 1 zone")
				require.Len(t, cfg.ZoneConfigs, 1, "want 1 zoneset")
				require.Empty(t, cfg.Warnings, "want 0 warnings")

				zone, ok := cfg.Zones["Storage-Zone@vsan10"]
				require.True(t, ok, "zone key 'Storage-Zone@vsan10' must exist")
				require.Len(t, zone.Members, 2, "zone must have 2 members")
				require.Equal(t, "alias", zone.Members[0].Type, "member[0] must be alias")
				require.Equal(t, "alias", zone.Members[1].Type, "member[1] must be alias")
			},
		},
		{
			name:    "multi-VSAN produces distinct zones",
			fixture: "multi_vsan.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Zones, 2, "want 2 zones (one per VSAN)")
				require.Len(t, cfg.ZoneConfigs, 2, "want 2 zonesets")

				_, okA := cfg.Zones["Zone-A@vsan10"]
				require.True(t, okA, "zone key 'Zone-A@vsan10' must exist")

				_, okB := cfg.Zones["Zone-B@vsan20"]
				require.True(t, okB, "zone key 'Zone-B@vsan20' must exist")
			},
		},
		{
			name:    "smart zoning keywords stripped with warning",
			fixture: "smart_zoning.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Zones, 1, "want 1 zone")
				require.Len(t, cfg.Warnings, 3, "want 3 warnings (one per smart-zoning role)")

				zone, ok := cfg.Zones["SmartZone@vsan10"]
				require.True(t, ok, "zone key 'SmartZone@vsan10' must exist")
				require.Len(t, zone.Members, 3, "zone must have 3 pwwn members")

				for i, m := range zone.Members {
					require.Equal(t, "pwwn", m.Type, "member[%d] must be pwwn", i)
				}

				for _, w := range cfg.Warnings {
					require.True(t, strings.Contains(w, "smart-zoning role"), "warning must contain 'smart-zoning role': %q", w)
				}
			},
		},
		{
			name:    "unsupported members skipped with warnings",
			fixture: "unsupported.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Warnings, 4, "want 4 warnings for unsupported members")

				zone, ok := cfg.Zones["Mixed@vsan10"]
				require.True(t, ok, "zone key 'Mixed@vsan10' must exist")

				// Count alias and unsupported members
				aliasCount := 0
				unsupportedCount := 0
				for _, m := range zone.Members {
					switch m.Type {
					case "alias":
						aliasCount++
					case "unsupported":
						unsupportedCount++
					}
				}
				require.Equal(t, 1, aliasCount, "want exactly 1 alias member")
				require.Equal(t, 0, unsupportedCount, "unsupported members must NOT be added to zone.Members")
				// Total members should be exactly 1 (only the alias member)
				require.Len(t, zone.Members, 1, "want exactly 1 member (unsupported ones are skipped)")
			},
		},
		{
			name:    "IVR zone skipped with warning",
			fixture: "edge_cases.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				// No key containing "IVR" in cfg.Zones
				for k := range cfg.Zones {
					require.False(t, strings.Contains(k, "IVR"), "IVR zone must not appear in cfg.Zones, but found key %q", k)
				}

				// At least one warning containing "IVR"
				ivrWarning := false
				for _, w := range cfg.Warnings {
					if strings.Contains(w, "IVR") {
						ivrWarning = true
						break
					}
				}
				require.True(t, ivrWarning, "at least one warning must contain 'IVR'")

				// EmptyZone present with 0 members
				emptyZone, ok := cfg.Zones["EmptyZone@vsan10"]
				require.True(t, ok, "zone key 'EmptyZone@vsan10' must exist")
				require.Empty(t, emptyZone.Members, "EmptyZone must have 0 members")

				// OrphanZone present
				_, orphanOk := cfg.Zones["OrphanZone@vsan10"]
				require.True(t, orphanOk, "zone key 'OrphanZone@vsan10' must exist")

				// ActiveZone present with 1 member
				activeZone, activeOk := cfg.Zones["ActiveZone@vsan10"]
				require.True(t, activeOk, "zone key 'ActiveZone@vsan10' must exist")
				require.Len(t, activeZone.Members, 1, "ActiveZone must have 1 member")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixturePath := filepath.Join("..", "..", "..", "testdata", "mds", tt.fixture)
			f, err := os.Open(fixturePath)
			require.NoError(t, err, "failed to open fixture %s", tt.fixture)
			defer f.Close() //nolint:errcheck

			cfg, err := Parse(f)
			require.NoError(t, err, "Parse must not return an error for %s", tt.fixture)
			require.NotNil(t, cfg, "Parse must return non-nil cfg")
			require.Equal(t, "mds-nxos", cfg.SourceFormat, "SourceFormat must be 'mds-nxos'")

			tt.checkFn(t, cfg)
		})
	}
}
