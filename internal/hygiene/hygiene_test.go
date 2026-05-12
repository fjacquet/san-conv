package hygiene

import (
	"strings"
	"testing"

	"github.com/fjacquet/san-conv/internal/ir"
	"github.com/stretchr/testify/require"
)

// helper: build a minimal *ir.ZoningConfig with empty maps.
func newCfg() *ir.ZoningConfig {
	return &ir.ZoningConfig{
		Aliases:     make(map[string]*ir.Alias),
		Zones:       make(map[string]*ir.Zone),
		ZoneConfigs: make(map[string]*ir.ZoneConfig),
	}
}

// helper: build an alias member.
func aliasMember(name string) *ir.ZoneMember {
	return &ir.ZoneMember{Type: "alias", Value: name}
}

// hasWarning returns true if any warning contains substr.
func hasWarning(warnings []string, substr string) bool {
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			return true
		}
	}
	return false
}

// countWarningsContaining returns the count of warnings containing substr.
func countWarningsContaining(warnings []string, substr string) int {
	n := 0
	for _, w := range warnings {
		if strings.Contains(w, substr) {
			n++
		}
	}
	return n
}

func TestCheck_DanglingAliasReference(t *testing.T) {
	t.Parallel()

	t.Run("dangling alias member emits one warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Zones["ZoneA@vsan10"] = &ir.Zone{
			Name: "ZoneA", VSAN: 10,
			Members: []*ir.ZoneMember{
				aliasMember("Missing"),
				aliasMember("Missing"), // two members pointing at same undefined alias
			},
		}
		// No aliases defined at all
		Check(cfg)
		// Each dangling member gets its own warning
		n := countWarningsContaining(cfg.Warnings, `member alias "Missing" is not defined — dangling reference`)
		require.Equal(t, 2, n, "expected one warning per dangling member")
	})

	t.Run("alias member defined in cfg.Aliases generates no dangling warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Zones["ZoneA@vsan10"] = &ir.Zone{
			Name: "ZoneA", VSAN: 10,
			Members: []*ir.ZoneMember{
				aliasMember("A"),
			},
		}
		Check(cfg)
		require.False(t, hasWarning(cfg.Warnings, "dangling reference"),
			"no dangling warning expected when alias is defined")
	})

	t.Run("mix of defined and undefined aliases", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["GoodAlias"] = &ir.Alias{Name: "GoodAlias", PWWN: "10:00:00:00:c9:00:00:02"}
		cfg.Zones["ZoneB@vsan20"] = &ir.Zone{
			Name: "ZoneB", VSAN: 20,
			Members: []*ir.ZoneMember{
				aliasMember("GoodAlias"),
				aliasMember("MissingAlias"),
			},
		}
		Check(cfg)
		n := countWarningsContaining(cfg.Warnings, `member alias "MissingAlias" is not defined — dangling reference`)
		require.Equal(t, 1, n)
		require.False(t, hasWarning(cfg.Warnings, `member alias "GoodAlias" is not defined`),
			"defined alias must not generate dangling warning")
	})
}

func TestCheck_EmptyZone(t *testing.T) {
	t.Parallel()

	t.Run("zone with no members emits warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Zones["Empty@vsan10"] = &ir.Zone{Name: "Empty", VSAN: 10, Members: []*ir.ZoneMember{}}
		Check(cfg)
		require.True(t, hasWarning(cfg.Warnings, `zone "Empty" has no members`),
			"expected empty-zone warning")
	})

	t.Run("zone with members does not emit empty warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Aliases["B"] = &ir.Alias{Name: "B", PWWN: "10:00:00:00:c9:00:00:02"}
		cfg.Zones["Full@vsan10"] = &ir.Zone{
			Name: "Full", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("B")},
		}
		Check(cfg)
		require.False(t, hasWarning(cfg.Warnings, `zone "Full" has no members`))
	})
}

func TestCheck_SingleMemberZone(t *testing.T) {
	t.Parallel()

	t.Run("zone with one member emits single-member warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Zones["Solo@vsan10"] = &ir.Zone{
			Name: "Solo", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A")},
		}
		Check(cfg)
		require.True(t, hasWarning(cfg.Warnings, `zone "Solo" has a single member — nothing to communicate with`),
			"expected single-member warning")
	})

	t.Run("zone with two members does not emit single-member warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Aliases["B"] = &ir.Alias{Name: "B", PWWN: "10:00:00:00:c9:00:00:02"}
		cfg.Zones["Pair@vsan10"] = &ir.Zone{
			Name: "Pair", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("B")},
		}
		Check(cfg)
		require.False(t, hasWarning(cfg.Warnings, `zone "Pair" has a single member`))
	})
}

func TestCheck_OrphanedAliases(t *testing.T) {
	t.Parallel()

	t.Run("single unused alias emits one aggregate warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Aliases["B"] = &ir.Alias{Name: "B", PWWN: "10:00:00:00:c9:00:00:02"}
		// Zone uses A but not B
		cfg.Zones["ZoneA@vsan10"] = &ir.Zone{
			Name: "ZoneA", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("A")},
		}
		Check(cfg)
		require.True(t, hasWarning(cfg.Warnings, `1 alias(es) defined but unused in any zone: B`),
			"expected orphaned-alias warning listing B")
		n := countWarningsContaining(cfg.Warnings, "alias(es) defined but unused")
		require.Equal(t, 1, n, "must be exactly one aggregate warning")
	})

	t.Run("all aliases used emits no orphaned-alias warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Zones["ZoneA@vsan10"] = &ir.Zone{
			Name: "ZoneA", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A")},
		}
		Check(cfg)
		require.False(t, hasWarning(cfg.Warnings, "alias(es) defined but unused"),
			"no orphaned-alias warning when all are used")
	})

	t.Run("13 unused aliases: lists 10 and shows (and 3 more)", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		// Add 13 aliases, all unused
		names := []string{
			"Alias01", "Alias02", "Alias03", "Alias04", "Alias05",
			"Alias06", "Alias07", "Alias08", "Alias09", "Alias10",
			"Alias11", "Alias12", "Alias13",
		}
		for i, n := range names {
			cfg.Aliases[n] = &ir.Alias{Name: n, PWWN: "10:00:00:00:c9:00:00:01"}
			_ = i
		}
		Check(cfg)
		n := countWarningsContaining(cfg.Warnings, "alias(es) defined but unused")
		require.Equal(t, 1, n, "must be one aggregate warning")
		require.True(t, hasWarning(cfg.Warnings, "(and 3 more)"),
			"truncation suffix (and 3 more) must appear")
		// First sorted name is Alias01
		require.True(t, hasWarning(cfg.Warnings, "Alias01"),
			"first sorted alias must appear in warning")
	})
}

func TestCheck_OrphanedZones(t *testing.T) {
	t.Parallel()

	t.Run("zone not in any zoneset emits one aggregate warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Aliases["B"] = &ir.Alias{Name: "B", PWWN: "10:00:00:00:c9:00:00:02"}
		// Zone "Lonely" not in any ZoneConfig
		cfg.Zones["Lonely@vsan10"] = &ir.Zone{
			Name: "Lonely", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("B")},
		}
		// Zone "InCfg" is referenced by a ZoneConfig
		cfg.Zones["InCfg@vsan10"] = &ir.Zone{
			Name: "InCfg", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("B")},
		}
		cfg.ZoneConfigs["MySet@vsan10"] = &ir.ZoneConfig{
			Name: "MySet", VSAN: 10,
			ZoneNames: []string{"InCfg"},
		}
		Check(cfg)
		require.True(t, hasWarning(cfg.Warnings, `1 zone(s) not in any zoneset: Lonely`),
			"expected orphaned-zone warning for Lonely")
		require.False(t, hasWarning(cfg.Warnings, "InCfg"),
			"InCfg is in a zoneset and must not appear in orphaned-zone warning")
	})

	t.Run("same-named zone in two VSANs both orphaned: name listed once, count is 1", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Aliases["B"] = &ir.Alias{Name: "B", PWWN: "10:00:00:00:c9:00:00:02"}
		cfg.Zones["Dup@vsan10"] = &ir.Zone{
			Name: "Dup", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("B")},
		}
		cfg.Zones["Dup@vsan20"] = &ir.Zone{
			Name: "Dup", VSAN: 20,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("B")},
		}
		// No ZoneConfigs → both zones are orphaned, but same .Name "Dup"
		Check(cfg)
		n := countWarningsContaining(cfg.Warnings, "zone(s) not in any zoneset")
		require.Equal(t, 1, n, "must be exactly one aggregate orphaned-zone warning")
		require.True(t, hasWarning(cfg.Warnings, `1 zone(s) not in any zoneset: Dup`),
			"Dup must appear once with count 1")
	})

	t.Run("all zones in a zoneset emits no orphaned-zone warning", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Aliases["B"] = &ir.Alias{Name: "B", PWWN: "10:00:00:00:c9:00:00:02"}
		cfg.Zones["ZoneX@vsan10"] = &ir.Zone{
			Name: "ZoneX", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A"), aliasMember("B")},
		}
		cfg.ZoneConfigs["SetX@vsan10"] = &ir.ZoneConfig{
			Name: "SetX", VSAN: 10,
			ZoneNames: []string{"ZoneX"},
		}
		Check(cfg)
		require.False(t, hasWarning(cfg.Warnings, "zone(s) not in any zoneset"),
			"no orphaned-zone warning when all zones are in a zoneset")
	})
}

func TestCheck_EmptyCfg(t *testing.T) {
	t.Parallel()

	t.Run("empty ZoningConfig does not panic and emits no warnings", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		require.NotPanics(t, func() { Check(cfg) })
		require.Empty(t, cfg.Warnings, "empty cfg must produce no warnings")
	})
}

func TestCheck_DoesNotMutate(t *testing.T) {
	t.Parallel()

	t.Run("Check does not modify zones, aliases, or zoneconfigs maps", func(t *testing.T) {
		t.Parallel()
		cfg := newCfg()
		cfg.Aliases["A"] = &ir.Alias{Name: "A", PWWN: "10:00:00:00:c9:00:00:01"}
		cfg.Zones["ZoneA@vsan10"] = &ir.Zone{
			Name: "ZoneA", VSAN: 10,
			Members: []*ir.ZoneMember{aliasMember("A")},
		}
		cfg.ZoneConfigs["Set1@vsan10"] = &ir.ZoneConfig{
			Name: "Set1", VSAN: 10,
			ZoneNames: []string{"ZoneA"},
		}
		nAliases := len(cfg.Aliases)
		nZones := len(cfg.Zones)
		nCfgs := len(cfg.ZoneConfigs)
		Check(cfg)
		require.Equal(t, nAliases, len(cfg.Aliases), "Check must not modify cfg.Aliases map")
		require.Equal(t, nZones, len(cfg.Zones), "Check must not modify cfg.Zones map")
		require.Equal(t, nCfgs, len(cfg.ZoneConfigs), "Check must not modify cfg.ZoneConfigs map")
	})
}
