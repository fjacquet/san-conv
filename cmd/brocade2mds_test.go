package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// resetBrocade2mdsFlags restores the brocade2mds command flags to their
// defaults. cobra's pflag.FlagSet does not reset between repeated Execute
// calls (it only assigns values for flags that appear in the new argv), so
// tests that drive the same command twice must reset explicitly to avoid
// state from the previous invocation leaking into the next one.
func resetBrocade2mdsFlags(t *testing.T) {
	t.Helper()
	flags := brocade2mdsCmd.Flags()
	require.NoError(t, flags.Set("output", ""))
	require.NoError(t, flags.Set("smart-consolidate", "false"))
	require.NoError(t, flags.Set("peer-consolidate", "false"))
	require.NoError(t, flags.Set("consolidate-strict", "false"))
	require.NoError(t, flags.Set("consolidate-report", ""))
}

// TestBrocade2MDS_PeerConsolidateAliasRegistration confirms the flag is
// registered, hidden in --help, and that --smart-consolidate remains visible
// as the canonical name.
func TestBrocade2MDS_PeerConsolidateAliasRegistration(t *testing.T) {
	peer := brocade2mdsCmd.Flags().Lookup("peer-consolidate")
	require.NotNil(t, peer, "--peer-consolidate must be registered as a hidden alias of --smart-consolidate")
	require.True(t, peer.Hidden, "--peer-consolidate should be hidden so --help advertises only the canonical --smart-consolidate")

	smart := brocade2mdsCmd.Flags().Lookup("smart-consolidate")
	require.NotNil(t, smart, "--smart-consolidate must remain registered as the canonical flag")
	require.False(t, smart.Hidden, "--smart-consolidate must stay visible in --help")
}

// TestBrocade2MDS_PeerConsolidateAliasIdenticalToSmartConsolidate drives the
// brocade2mds command end-to-end twice over the same flat-zones fixture —
// once with --smart-consolidate, once with --peer-consolidate — and asserts
// the resulting MDS output is byte-identical. This is the cobra-level
// regression that catches any future drift between the two spellings.
func TestBrocade2MDS_PeerConsolidateAliasIdenticalToSmartConsolidate(t *testing.T) {
	// Not parallel: rootCmd's flag state is package-level and other tests
	// using it would race.
	fixture, err := filepath.Abs(filepath.Join("..", "testdata", "brocade", "cli_flat_zones.cfg"))
	require.NoError(t, err)
	require.FileExists(t, fixture)

	tmp := t.TempDir()
	outSmart := filepath.Join(tmp, "smart.txt")
	outPeer := filepath.Join(tmp, "peer.txt")

	// Restore the rootCmd flag state when the test exits so other tests in
	// the same package see a clean slate.
	t.Cleanup(func() {
		resetBrocade2mdsFlags(t)
		rootCmd.SetArgs(nil)
	})

	// Run 1: --smart-consolidate (canonical).
	resetBrocade2mdsFlags(t)
	rootCmd.SetArgs([]string{"brocade2mds", "--smart-consolidate", "--output", outSmart, fixture})
	require.NoError(t, rootCmd.Execute(), "brocade2mds --smart-consolidate failed")

	// Run 2: --peer-consolidate (alias).
	resetBrocade2mdsFlags(t)
	rootCmd.SetArgs([]string{"brocade2mds", "--peer-consolidate", "--output", outPeer, fixture})
	require.NoError(t, rootCmd.Execute(), "brocade2mds --peer-consolidate failed — alias is broken")

	smartContent, err := os.ReadFile(outSmart)
	require.NoError(t, err)
	require.NotEmpty(t, smartContent, "--smart-consolidate produced empty output (fixture broke?)")

	peerContent, err := os.ReadFile(outPeer)
	require.NoError(t, err)

	require.Equal(t, string(smartContent), string(peerContent),
		"--peer-consolidate must produce byte-identical output to --smart-consolidate")
}
