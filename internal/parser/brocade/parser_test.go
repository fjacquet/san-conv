package brocade

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
			name:    "cfgshow basic with aliases zones and cfg",
			fixture: "cfgshow_basic.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Aliases, 4, "want 4 aliases (host_01, storage_01, host_02, storage_02)")
				require.Equal(t, "10:00:00:00:c9:ab:cd:ef", cfg.Aliases["host_01"].PWWN, "host_01 PWWN mismatch")
				require.Equal(t, 0, cfg.Aliases["host_01"].VSAN, "host_01 VSAN must be 0 (Brocade sentinel)")
				require.Equal(t, "50:05:07:61:01:23:45:67", cfg.Aliases["storage_01"].PWWN, "storage_01 PWWN mismatch")
				require.Len(t, cfg.Zones, 2, "want 2 zones")

				zone1, ok := cfg.Zones["fabric_zone1"]
				require.True(t, ok, "zone 'fabric_zone1' must exist")
				require.Equal(t, 0, zone1.VSAN, "fabric_zone1 VSAN must be 0")
				require.Len(t, zone1.Members, 2, "fabric_zone1 must have 2 members")
				require.Equal(t, "alias", zone1.Members[0].Type, "fabric_zone1 member[0] must be alias")
				require.Equal(t, "host_01", zone1.Members[0].Value, "fabric_zone1 member[0] value mismatch")
				require.Equal(t, "alias", zone1.Members[1].Type, "fabric_zone1 member[1] must be alias")
				require.Equal(t, "storage_01", zone1.Members[1].Value, "fabric_zone1 member[1] value mismatch")

				require.Len(t, cfg.ZoneConfigs, 1, "want 1 cfg")
				prodCfg, ok := cfg.ZoneConfigs["Production_cfg"]
				require.True(t, ok, "cfg 'Production_cfg' must exist")
				require.Equal(t, 0, prodCfg.VSAN, "Production_cfg VSAN must be 0")
				require.Equal(t, []string{"fabric_zone1", "fabric_zone2"}, prodCfg.ZoneNames, "Production_cfg ZoneNames mismatch")

				require.Empty(t, cfg.Warnings, "want 0 warnings")
			},
		},
		{
			name:    "cfgshow backslash continuation",
			fixture: "cfgshow_continuation.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Aliases, 5, "want 5 aliases")
				require.Len(t, cfg.Zones, 2, "want 2 zones")

				bigZone, ok := cfg.Zones["big_zone"]
				require.True(t, ok, "zone 'big_zone' must exist")
				// CRITICAL: continuation test — big_zone must have 4 members, NOT 2
				require.Len(t, bigZone.Members, 4, "big_zone must have 4 members (backslash continuation must be handled)")
				require.Equal(t, "member_01", bigZone.Members[0].Value, "big_zone member[0] value mismatch")
				require.Equal(t, "member_02", bigZone.Members[1].Value, "big_zone member[1] value mismatch")
				require.Equal(t, "member_03", bigZone.Members[2].Value, "big_zone member[2] value mismatch")
				require.Equal(t, "member_04", bigZone.Members[3].Value, "big_zone member[3] value mismatch")

				smallZone, ok := cfg.Zones["small_zone"]
				require.True(t, ok, "zone 'small_zone' must exist")
				require.Len(t, smallZone.Members, 1, "small_zone must have exactly 1 member")

				bigFabricCfg, ok := cfg.ZoneConfigs["BigFabric_cfg"]
				require.True(t, ok, "cfg 'BigFabric_cfg' must exist")
				require.Equal(t, []string{"big_zone", "small_zone"}, bigFabricCfg.ZoneNames, "BigFabric_cfg ZoneNames mismatch")

				require.Empty(t, cfg.Warnings, "want 0 warnings")
			},
		},
		{
			name:    "CLI basic with alicreate zonecreate cfgcreate",
			fixture: "cli_basic.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Len(t, cfg.Aliases, 2, "want 2 aliases")
				require.Equal(t, "10:00:00:00:c9:ab:cd:ef", cfg.Aliases["host_01"].PWWN, "host_01 PWWN mismatch")

				require.Len(t, cfg.Zones, 1, "want 1 zone")
				zone, ok := cfg.Zones["fabric_zone1"]
				require.True(t, ok, "zone 'fabric_zone1' must exist")
				require.Len(t, zone.Members, 2, "fabric_zone1 must have 2 members")
				require.Equal(t, "alias", zone.Members[0].Type, "fabric_zone1 member[0] must be alias")
				require.Equal(t, "host_01", zone.Members[0].Value, "fabric_zone1 member[0] value mismatch")
				require.Equal(t, "alias", zone.Members[1].Type, "fabric_zone1 member[1] must be alias")
				require.Equal(t, "storage_01", zone.Members[1].Value, "fabric_zone1 member[1] value mismatch")

				require.Len(t, cfg.ZoneConfigs, 1, "want 1 cfg")
				prodCfg, ok := cfg.ZoneConfigs["Production_cfg"]
				require.True(t, ok, "cfg 'Production_cfg' must exist")
				require.Equal(t, []string{"fabric_zone1"}, prodCfg.ZoneNames, "Production_cfg ZoneNames mismatch")

				require.Empty(t, cfg.Warnings, "want 0 warnings")
			},
		},
		{
			name:    "CLI pWWN members in zonecreate",
			fixture: "cli_pwwn_members.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				require.Empty(t, cfg.Aliases, "want 0 aliases")
				require.Len(t, cfg.Zones, 1, "want 1 zone")

				zone, ok := cfg.Zones["direct_zone"]
				require.True(t, ok, "zone 'direct_zone' must exist")
				require.Len(t, zone.Members, 2, "direct_zone must have 2 members")
				require.Equal(t, "pwwn", zone.Members[0].Type, "direct_zone member[0] must be pwwn")
				require.Equal(t, "10:00:00:00:c9:ab:cd:ef", zone.Members[0].Value, "direct_zone member[0] value mismatch")
				require.Equal(t, "pwwn", zone.Members[1].Type, "direct_zone member[1] must be pwwn")
				require.Equal(t, "50:05:07:61:01:23:45:67", zone.Members[1].Value, "direct_zone member[1] value mismatch")

				require.Len(t, cfg.ZoneConfigs, 1, "want 1 cfg")

				require.Empty(t, cfg.Warnings, "want 0 warnings")
			},
		},
		{
			name:    "edge cases empty zone and Effective section boundary",
			fixture: "edge_cases.cfg",
			checkFn: func(t *testing.T, cfg *ir.ZoningConfig) {
				t.Helper()
				// CRITICAL: assert counts confirm Effective section was NOT parsed (would double counts)
				require.Len(t, cfg.Aliases, 1, "want 1 alias (Effective section must not be parsed)")
				require.Equal(t, "10:00:00:00:c9:ff:ff:01", cfg.Aliases["host_a"].PWWN, "host_a PWWN mismatch")

				require.Len(t, cfg.Zones, 2, "want 2 zones (Effective section must not be parsed)")

				emptyZone, ok := cfg.Zones["empty_zone"]
				require.True(t, ok, "zone 'empty_zone' must exist")
				require.Empty(t, emptyZone.Members, "empty_zone must have 0 members")

				realZone, ok := cfg.Zones["real_zone"]
				require.True(t, ok, "zone 'real_zone' must exist")
				require.Len(t, realZone.Members, 1, "real_zone must have 1 member")
				require.Equal(t, "alias", realZone.Members[0].Type, "real_zone member[0] must be alias")
				require.Equal(t, "host_a", realZone.Members[0].Value, "real_zone member[0] value mismatch")

				require.Len(t, cfg.ZoneConfigs, 1, "want 1 cfg (Effective section must not be parsed)")
				testCfg, ok := cfg.ZoneConfigs["Test_cfg"]
				require.True(t, ok, "cfg 'Test_cfg' must exist")
				require.Equal(t, []string{"empty_zone", "real_zone"}, testCfg.ZoneNames, "Test_cfg ZoneNames mismatch")

				require.Empty(t, cfg.Warnings, "want 0 warnings")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fixturePath := filepath.Join("..", "..", "..", "testdata", "brocade", tt.fixture)
			f, err := os.Open(fixturePath)
			require.NoError(t, err, "failed to open fixture %s", tt.fixture)
			defer f.Close() //nolint:errcheck

			cfg, err := Parse(f)
			require.NoError(t, err, "Parse must not return an error for %s", tt.fixture)
			require.NotNil(t, cfg, "Parse must return non-nil cfg")
			require.Equal(t, "brocade-fos", cfg.SourceFormat, "SourceFormat must be 'brocade-fos'")

			tt.checkFn(t, cfg)
		})
	}
}

func TestParse_TerminalArtifacts(t *testing.T) {
	t.Parallel()

	// Brocade "cfgshow" output captured through a paged terminal session: an
	// ANSI-wrapped --More-- prompt, followed by a bare CR, glued onto the start
	// of the line that scrolled in after it — once before a "zone:" token, once
	// before an "alias:" token. preprocess.Clean must un-glue these so the
	// parser still sees a clean cfgshow listing.
	const captured = "Defined configuration:\r\n" +
		" cfg:   Prod_cfg\r\n" +
		"        z1; z2\r\n" +
		" zone:  z1\r\n" +
		"        host_01; storage_01\r\n" +
		"\x1b[7m--More--\x1b[m\r zone:  z2\r\n" +
		"        host_02; storage_02\r\n" +
		" alias: host_01\r\n" +
		"        10:00:00:00:c9:ab:cd:ef\r\n" +
		" alias: storage_01\r\n" +
		"        50:05:07:61:01:23:45:67\r\n" +
		"\x1b[7m--More--\x1b[m\r alias: host_02\r\n" +
		"        10:00:00:00:c9:ab:cd:ee\r\n" +
		" alias: storage_02\r\n" +
		"        50:05:07:61:01:23:45:68\r\n"

	cfg, err := Parse(strings.NewReader(captured))
	require.NoError(t, err)

	// z2's "zone:" line was glued behind a --More-- prompt; it and its members must survive.
	require.Len(t, cfg.Zones, 2, "both zones must survive the --More-- prompts")
	z2, ok := cfg.Zones["z2"]
	require.True(t, ok, "zone 'z2' (glued behind --More--) must exist")
	require.Len(t, z2.Members, 2, "z2 must keep both members")
	require.Equal(t, "alias", z2.Members[0].Type)
	require.Equal(t, "host_02", z2.Members[0].Value)
	require.Equal(t, "alias", z2.Members[1].Type)
	require.Equal(t, "storage_02", z2.Members[1].Value)

	// host_02's "alias:" line was also glued behind a --More-- prompt.
	require.Len(t, cfg.Aliases, 4, "all four aliases must survive")
	require.Equal(t, "10:00:00:00:c9:ab:cd:ee", cfg.Aliases["host_02"].PWWN)

	require.Equal(t, []string{"z1", "z2"}, cfg.ZoneConfigs["Prod_cfg"].ZoneNames)
}
