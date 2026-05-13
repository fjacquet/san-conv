package consolidator

import (
	"testing"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/stretchr/testify/require"
)

// helper: build a minimal ZoningConfig with the given zones.
func buildCfg(zones map[string]*ir.Zone) *ir.ZoningConfig {
	return &ir.ZoningConfig{
		Aliases:      make(map[string]*ir.Alias),
		Zones:        zones,
		ZoneConfigs:  make(map[string]*ir.ZoneConfig),
		SourceFormat: "mds-nxos",
	}
}

// helper: 2-member alias-only zone, no roles.
func flatZone(name string, vsan int, m1, m2 string) *ir.Zone {
	return &ir.Zone{
		Name: name,
		VSAN: vsan,
		Members: []*ir.ZoneMember{
			{Type: "alias", Value: m1, Role: ""},
			{Type: "alias", Value: m2, Role: ""},
		},
	}
}

// ─── Happy path ──────────────────────────────────────────────────────────────

// Three flat zones sharing the same target TGT1 → peer zone created.
func TestConsolidate_HappyPath(t *testing.T) {
	t.Parallel()

	cfg := buildCfg(map[string]*ir.Zone{
		"ESX1_TGT1@vsan10": flatZone("ESX1_TGT1", 10, "ESX1", "TGT1"),
		"ESX2_TGT1@vsan10": flatZone("ESX2_TGT1", 10, "ESX2", "TGT1"),
		"ESX3_TGT1@vsan10": flatZone("ESX3_TGT1", 10, "ESX3", "TGT1"),
	})

	report := Consolidate(cfg, false, "peerzone")

	// Source zones must be removed.
	require.NotContains(t, cfg.Zones, "ESX1_TGT1@vsan10")
	require.NotContains(t, cfg.Zones, "ESX2_TGT1@vsan10")
	require.NotContains(t, cfg.Zones, "ESX3_TGT1@vsan10")

	// Peer zone must be present.
	pz, ok := cfg.Zones["TGT1_peerzone@vsan10"]
	require.True(t, ok, "TGT1_peerzone@vsan10 must exist in cfg.Zones")
	require.Equal(t, "TGT1_peerzone", pz.Name)
	require.Equal(t, 10, pz.VSAN)

	// Members: target first, then inits in sorted (first-seen) order.
	require.Len(t, pz.Members, 4)
	require.Equal(t, "alias", pz.Members[0].Type)
	require.Equal(t, "TGT1", pz.Members[0].Value)
	require.Equal(t, "target", pz.Members[0].Role)
	for _, m := range pz.Members[1:] {
		require.Equal(t, "init", m.Role)
		require.Equal(t, "alias", m.Type)
	}
	initNames := []string{pz.Members[1].Value, pz.Members[2].Value, pz.Members[3].Value}
	require.ElementsMatch(t, []string{"ESX1", "ESX2", "ESX3"}, initNames)

	// Report.
	require.Len(t, report.Zones, 1)
	czs := report.Zones[0]
	require.Equal(t, "TGT1", czs.Target)
	require.Equal(t, "TGT1_peerzone", czs.NewName)
	require.Equal(t, 10, czs.VSAN)
	require.ElementsMatch(t, []string{"ESX1", "ESX2", "ESX3"}, czs.Members)
	require.Equal(t, []string{"ESX1_TGT1", "ESX2_TGT1", "ESX3_TGT1"}, czs.SourceZones) // sorted
	require.Empty(t, report.Skipped)
}

// ─── ZoneConfigs rewrite ─────────────────────────────────────────────────────

// ZoneConfig referencing two source zones + an unrelated zone → peer name deduplicated.
func TestConsolidate_ZoneConfigsRewrite(t *testing.T) {
	t.Parallel()

	cfg := buildCfg(map[string]*ir.Zone{
		"ESX1_TGT1@vsan10": flatZone("ESX1_TGT1", 10, "ESX1", "TGT1"),
		"ESX2_TGT1@vsan10": flatZone("ESX2_TGT1", 10, "ESX2", "TGT1"),
		"ESX3_TGT1@vsan10": flatZone("ESX3_TGT1", 10, "ESX3", "TGT1"),
		"SomeOtherZone@vsan10": {
			Name: "SomeOtherZone", VSAN: 10,
			Members: []*ir.ZoneMember{
				{Type: "alias", Value: "A", Role: ""},
				{Type: "alias", Value: "B", Role: ""},
				{Type: "alias", Value: "C", Role: ""},
			},
		},
	})
	cfg.ZoneConfigs["cfg1@vsan10"] = &ir.ZoneConfig{
		Name:      "cfg1",
		VSAN:      10,
		ZoneNames: []string{"ESX1_TGT1", "ESX2_TGT1", "SomeOtherZone"},
	}

	Consolidate(cfg, false, "peerzone")

	zc := cfg.ZoneConfigs["cfg1@vsan10"]
	require.NotNil(t, zc)
	// ESX1_TGT1 and ESX2_TGT1 both map to TGT1_peerzone; SomeOtherZone stays.
	// ESX3_TGT1 was NOT in this config so doesn't add another TGT1_peerzone.
	require.Equal(t, []string{"TGT1_peerzone", "SomeOtherZone"}, zc.ZoneNames)
}

// ─── Single host → left flat ─────────────────────────────────────────────────

// Only one initiator for a target → nothing to consolidate.
func TestConsolidate_SingleHost_LeftFlat(t *testing.T) {
	t.Parallel()

	cfg := buildCfg(map[string]*ir.Zone{
		"ESX1_TGT9@vsan10": flatZone("ESX1_TGT9", 10, "ESX1", "TGT9"),
	})

	report := Consolidate(cfg, false, "peerzone")

	// Source zone must remain.
	require.Contains(t, cfg.Zones, "ESX1_TGT9@vsan10")
	// No peer zone created.
	require.NotContains(t, cfg.Zones, "TGT9_peerzone@vsan10")
	// No Zones in report.
	require.Empty(t, report.Zones)
	// Skipped with a reason mentioning infrequent target or only one host.
	require.Len(t, report.Skipped, 1)
	require.Equal(t, "ESX1_TGT9", report.Skipped[0].Name)
	require.True(t,
		containsAny(report.Skipped[0].Reason, "only one host", "appears in only", "inferred target"),
		"reason should indicate not enough zones to consolidate, got: %s", report.Skipped[0].Reason,
	)
}

// ─── Name doesn't decompose → left flat ──────────────────────────────────────

// Zone name "RA1_RA2_SRDF" cannot be decomposed to "RA1"+"RA2".
func TestConsolidate_NameDoesNotDecompose(t *testing.T) {
	t.Parallel()

	cfg := buildCfg(map[string]*ir.Zone{
		"RA1_RA2_SRDF@vsan10": flatZone("RA1_RA2_SRDF", 10, "RA1", "RA2"),
	})

	report := Consolidate(cfg, false, "peerzone")

	require.Contains(t, cfg.Zones, "RA1_RA2_SRDF@vsan10")
	require.Empty(t, report.Zones)
	require.Len(t, report.Skipped, 1)
	require.Equal(t, "RA1_RA2_SRDF", report.Skipped[0].Name)
	require.Contains(t, report.Skipped[0].Reason, "trailing component")
}

// ─── Frequency veto — target too infrequent ───────────────────────────────────

// BIG_small: name says init=BIG target=small, but "small" only appears 1 time
// while "BIG" appears in 5 zones → frequency veto.
func TestConsolidate_FrequencyVeto_TargetTooInfrequent(t *testing.T) {
	t.Parallel()

	// Build 5 zones that all have "BIG" but different "smalls".
	// Plus the one zone that has name "BIG_small" so name decomposition says target=small.
	// The zone "BIG_small" has freq[small]=1, freq[BIG]=6 (BIG_small + 5 others).
	zones := map[string]*ir.Zone{
		"BIG_small@vsan10": flatZone("BIG_small", 10, "BIG", "small"),
		"BIG_TGT2@vsan10":  flatZone("BIG_TGT2", 10, "BIG", "TGT2"),
		"BIG_TGT3@vsan10":  flatZone("BIG_TGT3", 10, "BIG", "TGT3"),
		"BIG_TGT4@vsan10":  flatZone("BIG_TGT4", 10, "BIG", "TGT4"),
		"BIG_TGT5@vsan10":  flatZone("BIG_TGT5", 10, "BIG", "TGT5"),
		"BIG_TGT6@vsan10":  flatZone("BIG_TGT6", 10, "BIG", "TGT6"),
	}
	cfg := buildCfg(zones)

	report := Consolidate(cfg, false, "peerzone")

	// BIG_small must be left flat (frequency veto on "small").
	require.Contains(t, cfg.Zones, "BIG_small@vsan10")

	// Find the skipped entry for BIG_small.
	var foundSkip *SkippedZone
	for i := range report.Skipped {
		if report.Skipped[i].Name == "BIG_small" {
			foundSkip = &report.Skipped[i]
			break
		}
	}
	require.NotNil(t, foundSkip, "BIG_small must appear in report.Skipped")
	// Should mention frequency issue (either freq<2 or freq(target)<freq(init)).
	require.True(t,
		containsAny(foundSkip.Reason, "inferred target", "appears in only", "fewer zones"),
		"reason should mention frequency issue, got: %s", foundSkip.Reason,
	)
}

// ─── Non-candidate zones — completely untouched ───────────────────────────────

// Zones that are NOT 2-member alias-only roleless must be left alone and NOT in Report.
func TestConsolidate_NonCandidates_Untouched(t *testing.T) {
	t.Parallel()

	cfg := buildCfg(map[string]*ir.Zone{
		// 3-member zone.
		"3member@vsan10": {
			Name: "3member", VSAN: 10,
			Members: []*ir.ZoneMember{
				{Type: "alias", Value: "A", Role: ""},
				{Type: "alias", Value: "B", Role: ""},
				{Type: "alias", Value: "C", Role: ""},
			},
		},
		// 1-member zone.
		"1member@vsan10": {
			Name: "1member", VSAN: 10,
			Members: []*ir.ZoneMember{
				{Type: "alias", Value: "A", Role: ""},
			},
		},
		// Zone with a pwwn member.
		"pwwnzone@vsan10": {
			Name: "pwwnzone", VSAN: 10,
			Members: []*ir.ZoneMember{
				{Type: "pwwn", Value: "10:00:00:00:c9:12:34:56", Role: ""},
				{Type: "alias", Value: "B", Role: ""},
			},
		},
		// Zone with a member already carrying Role:"target" (smart-zoned).
		"smartzone@vsan10": {
			Name: "smartzone", VSAN: 10,
			Members: []*ir.ZoneMember{
				{Type: "alias", Value: "A", Role: "target"},
				{Type: "alias", Value: "B", Role: "init"},
			},
		},
	})

	report := Consolidate(cfg, false, "peerzone")

	// All four zones still present.
	require.Contains(t, cfg.Zones, "3member@vsan10")
	require.Contains(t, cfg.Zones, "1member@vsan10")
	require.Contains(t, cfg.Zones, "pwwnzone@vsan10")
	require.Contains(t, cfg.Zones, "smartzone@vsan10")

	// None in the report at all.
	require.Empty(t, report.Zones)
	require.Empty(t, report.Skipped)
}

// ─── Determinism ─────────────────────────────────────────────────────────────

// Two calls on equivalent inputs must yield identical reports.
func TestConsolidate_Determinism(t *testing.T) {
	t.Parallel()

	buildInput := func() *ir.ZoningConfig {
		return buildCfg(map[string]*ir.Zone{
			"ESX1_TGT1@vsan10":    flatZone("ESX1_TGT1", 10, "ESX1", "TGT1"),
			"ESX2_TGT1@vsan10":    flatZone("ESX2_TGT1", 10, "ESX2", "TGT1"),
			"ESX3_TGT1@vsan10":    flatZone("ESX3_TGT1", 10, "ESX3", "TGT1"),
			"RA1_RA2_SRDF@vsan10": flatZone("RA1_RA2_SRDF", 10, "RA1", "RA2"),
		})
	}

	r1 := Consolidate(buildInput(), false, "peerzone")
	r2 := Consolidate(buildInput(), false, "peerzone")

	require.Equal(t, r1.Zones, r2.Zones)
	require.Equal(t, r1.Skipped, r2.Skipped)
}

// ─── Empty input ─────────────────────────────────────────────────────────────

func TestConsolidate_EmptyInput(t *testing.T) {
	t.Parallel()
	cfg := buildCfg(map[string]*ir.Zone{})
	report := Consolidate(cfg, false, "peerzone")
	require.Empty(t, report.Zones)
	require.Empty(t, report.Skipped)
}

// ─── Relaxed (default) — target = trailing component of the zone name ────────

// Zone names like "ESX9_HBA0_TGT1" (host alias is "ESX9", not "ESX9_HBA0") do
// not strictly decompose, but the target "TGT1" is a trailing component → the
// default (relaxed) mode consolidates them; --consolidate-strict does not.
func TestConsolidate_RelaxedTrailingComponent(t *testing.T) {
	t.Parallel()

	mk := func() *ir.ZoningConfig {
		return buildCfg(map[string]*ir.Zone{
			"ESX9_HBA0_TGT1@vsan10":  flatZone("ESX9_HBA0_TGT1", 10, "ESX9", "TGT1"),
			"ESX10_HBA0_TGT1@vsan10": flatZone("ESX10_HBA0_TGT1", 10, "ESX10", "TGT1"),
		})
	}

	t.Run("relaxed default consolidates by trailing-component target", func(t *testing.T) {
		t.Parallel()
		cfg := mk()
		report := Consolidate(cfg, false, "peerzone")
		require.NotContains(t, cfg.Zones, "ESX9_HBA0_TGT1@vsan10")
		require.NotContains(t, cfg.Zones, "ESX10_HBA0_TGT1@vsan10")
		pz, ok := cfg.Zones["TGT1_peerzone@vsan10"]
		require.True(t, ok, "TGT1_peerzone@vsan10 must exist")
		require.Equal(t, "TGT1", pz.Members[0].Value)
		require.Equal(t, "target", pz.Members[0].Role)
		require.Len(t, report.Zones, 1)
		require.Equal(t, "TGT1", report.Zones[0].Target)
		require.ElementsMatch(t, []string{"ESX9", "ESX10"}, report.Zones[0].Members)
		require.Empty(t, report.Skipped)
	})

	t.Run("strict leaves <host>_HBA0_<target> flat", func(t *testing.T) {
		t.Parallel()
		cfg := mk()
		report := Consolidate(cfg, true, "peerzone")
		require.Contains(t, cfg.Zones, "ESX9_HBA0_TGT1@vsan10")
		require.Contains(t, cfg.Zones, "ESX10_HBA0_TGT1@vsan10")
		require.NotContains(t, cfg.Zones, "TGT1_peerzone@vsan10")
		require.Empty(t, report.Zones)
		require.Len(t, report.Skipped, 2)
		for _, s := range report.Skipped {
			require.Contains(t, s.Reason, "--consolidate-strict")
		}
	})
}

// Both members are trailing components of the zone name → ambiguous, left flat.
func TestConsolidate_BothMembersTrailing_LeftFlat(t *testing.T) {
	t.Parallel()
	cfg := buildCfg(map[string]*ir.Zone{
		"Y_X@vsan10": flatZone("Y_X", 10, "X", "Y_X"),
	})
	report := Consolidate(cfg, false, "peerzone")
	require.Contains(t, cfg.Zones, "Y_X@vsan10")
	require.Empty(t, report.Zones)
	require.Len(t, report.Skipped, 1)
	require.Contains(t, report.Skipped[0].Reason, "both members are a trailing component")
}

func TestConsolidate_HonorsNameSuffix(t *testing.T) {
	t.Parallel()
	cfg := &ir.ZoningConfig{
		Aliases: map[string]*ir.Alias{
			"TGT1": {Name: "TGT1", PWWN: "50:00:00:00:00:00:00:01"},
			"ESX1": {Name: "ESX1", PWWN: "10:00:00:00:00:00:00:01"},
			"ESX2": {Name: "ESX2", PWWN: "10:00:00:00:00:00:00:02"},
		},
		Zones: map[string]*ir.Zone{
			"ESX1_TGT1@vsan0": {Name: "ESX1_TGT1", VSAN: 0, Members: []*ir.ZoneMember{
				{Type: "alias", Value: "ESX1"}, {Type: "alias", Value: "TGT1"},
			}},
			"ESX2_TGT1@vsan0": {Name: "ESX2_TGT1", VSAN: 0, Members: []*ir.ZoneMember{
				{Type: "alias", Value: "ESX2"}, {Type: "alias", Value: "TGT1"},
			}},
		},
	}
	report := Consolidate(cfg, false, "smartzone")
	require.Len(t, report.Zones, 1, "exactly one consolidated zone")
	require.Equal(t, "TGT1_smartzone", report.Zones[0].NewName)
	_, ok := cfg.Zones["TGT1_smartzone@vsan0"]
	require.True(t, ok, "consolidated zone present in cfg.Zones under suffix-based key")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
