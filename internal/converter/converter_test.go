package converter

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test 1: mds2brocade basic stdout — alicreate, zonecreate, and Summary: all appear.
func TestRun_MDS2Brocade_BasicStdout(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/basic.cfg",
		Direction:  "mds2brocade",
		FOSVersion: "pre-8.1",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "alicreate")
	require.Contains(t, stdout.String(), "zonecreate")
	require.Contains(t, stderr.String(), "Summary:")
}

// Test 2: mds2brocade output file — alicreate in file, stdout empty.
func TestRun_MDS2Brocade_OutputFile(t *testing.T) {
	t.Parallel()
	outFile := filepath.Join(t.TempDir(), "out.txt")
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/basic.cfg",
		Direction:  "mds2brocade",
		OutputFile: outFile,
		FOSVersion: "pre-8.1",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)

	// out.txt must exist and contain alicreate
	content, readErr := os.ReadFile(outFile)
	require.NoError(t, readErr, "output file must exist after conversion")
	require.Contains(t, string(content), "alicreate")

	// stdout must be empty — output was redirected to the file
	require.Empty(t, stdout.String(), "stdout must be empty when --output file is specified")
}

// Test 3: mds2brocade script file — defzone preamble, cfgsave, and executable permission.
func TestRun_MDS2Brocade_ScriptFile(t *testing.T) {
	t.Parallel()
	scriptFile := filepath.Join(t.TempDir(), "out.sh")
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/basic.cfg",
		Direction:  "mds2brocade",
		ScriptFile: scriptFile,
		FOSVersion: "pre-8.1",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)

	// Script file must exist
	info, statErr := os.Stat(scriptFile)
	require.NoError(t, statErr, "script file must be created")

	// Script file must be executable
	require.NotZero(t, info.Mode()&0o111, "script file must have executable permission bits set")

	// Script file must contain FOS script preamble and postamble
	content, readErr := os.ReadFile(scriptFile)
	require.NoError(t, readErr)
	require.Contains(t, string(content), "defzone --noaccess")
	require.Contains(t, string(content), "cfgsave")
}

// Test 4: mds2brocade script does NOT double-count warnings in stderr.
// Summary: must appear exactly once even when a script file is written.
func TestRun_MDS2Brocade_SummaryNotDoubled(t *testing.T) {
	t.Parallel()
	scriptFile := filepath.Join(t.TempDir(), "out.sh")
	var stdout, stderr bytes.Buffer
	// edge_cases.cfg produces sanitizer warnings (IVR zone, empty zone)
	opts := Options{
		InputFile:  "../../testdata/mds/edge_cases.cfg",
		Direction:  "mds2brocade",
		ScriptFile: scriptFile,
		FOSVersion: "pre-8.1",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)

	// The "Summary:" line must appear exactly once in stderr
	summaryCount := strings.Count(stderr.String(), "Summary:")
	require.Equal(t, 1, summaryCount, "Summary: must appear exactly once in stderr (not doubled by script emit)")
}

// Test 5: brocade2mds basic stdout — device-alias and zone name in output.
func TestRun_Brocade2MDS_BasicStdout(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "../../testdata/brocade/cfgshow_basic.cfg",
		Direction: "brocade2mds",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), "device-alias database")
	require.Contains(t, stdout.String(), "zone name")
}

// Test 6: brocade2mds sanitizer NOT applied — hyphens in alias names must survive.
func TestRun_Brocade2MDS_HyphenSurvives(t *testing.T) {
	t.Parallel()

	// Write a cfgshow fixture with a hyphenated alias name to a temp file
	hyphenFixture := `Defined configuration:
 cfg:   prod-cfg
                server-zone
 zone:  server-zone
                host-01; storage-01
 alias: host-01
                10:00:00:00:c9:ab:cd:ef
 alias: storage-01
                50:05:07:61:01:23:45:67
`
	fixtureFile := filepath.Join(t.TempDir(), "hyphen.cfg")
	require.NoError(t, os.WriteFile(fixtureFile, []byte(hyphenFixture), 0o644))

	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: fixtureFile,
		Direction: "brocade2mds",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)

	// Hyphen in alias name must survive — sanitizer must NOT be applied for brocade2mds
	require.Contains(t, stdout.String(), "host-01",
		"hyphenated alias must NOT be renamed to host_01 in brocade2mds direction")
	require.NotContains(t, stdout.String(), "host_01",
		"underscore replacement must not happen in brocade2mds direction")
}

// Test 7: --fos-version 8.1+ is accepted and does not cause an error.
func TestRun_FOSVersionFlag_Accepted(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/basic.cfg",
		Direction:  "mds2brocade",
		FOSVersion: "8.1+",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err, "--fos-version 8.1+ must be accepted without error")
}

// Test 8: missing input file returns non-nil error (CLI-06 fatal IO path).
func TestRun_MissingInputFile_ReturnsError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "nonexistent_file_xyz.cfg",
		Direction: "mds2brocade",
	}
	err := Run(opts, &stdout, &stderr)
	require.Error(t, err, "missing input file must return a non-nil error")
}

// Test 9: unknown direction returns non-nil error.
func TestRun_UnknownDirection_ReturnsError(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "../../testdata/mds/basic.cfg",
		Direction: "sideways",
	}
	err := Run(opts, &stdout, &stderr)
	require.Error(t, err, "unknown direction must return a non-nil error")
}

// VSAN filter: only the requested VSAN's zones appear in the output.
// Note: pre-8.1 sanitizer converts hyphens to underscores, so Zone-A → Zone_A.
func TestRun_MDS2Brocade_VSANFilter(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/multi_vsan.cfg",
		Direction:  "mds2brocade",
		FOSVersion: "pre-8.1",
		VSAN:       10,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	require.Contains(t, out, "Zone_A", "VSAN 10 zone must be present (sanitized name)")
	require.NotContains(t, out, "Zone_B", "VSAN 20 zone must be filtered out")
	require.NotContains(t, out, "ZS_VSAN20", "VSAN 20 zoneset must be filtered out")
	// The multi-VSAN warning's "pass --vsan N" advice is rewritten once we've scoped.
	require.Contains(t, stderr.String(), "converted only VSAN 10 (--vsan)")
	require.NotContains(t, stderr.String(), "pass --vsan N to scope")
}

// VSAN filter with no matching VSAN: warns and produces no zones, no error.
func TestRun_MDS2Brocade_VSANFilterNoMatch(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/multi_vsan.cfg",
		Direction:  "mds2brocade",
		FOSVersion: "pre-8.1",
		VSAN:       999,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stderr.String(), "--vsan 999 matched no zones")
	require.NotContains(t, stdout.String(), "zonecreate")
}

// Smart-zoned MDS zone converts to a Brocade peer zone (no "no FOS equivalent" warning).
func TestRun_MDS2Brocade_PeerZone(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "../../testdata/mds/smart_zoning.cfg",
		Direction: "mds2brocade",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	require.Contains(t, out, `zonecreate --peerzone "SmartZone" -principal `)
	require.Contains(t, out, `zonecreate --peerzone "SmartAliases" -principal `)
	require.NotContains(t, stderr.String(), "no FOS equivalent")
}

// A smart-zoned zone with only init members falls back to a plain zone + warning.
func TestRun_MDS2Brocade_PeerZoneInitOnlyFallback(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "../../testdata/mds/smart_zoning_initonly.cfg",
		Direction: "mds2brocade",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stdout.String(), `zonecreate "InitOnly", "10:00:00:00:c9:aa:00:01;10:00:00:00:c9:aa:00:02"`)
	require.NotContains(t, stdout.String(), "--peerzone")
	require.Contains(t, stderr.String(), `zone "InitOnly": peer zone has no principal members after resolution`)
}

// Test 10: stderr summary line format matches expected pattern.
// Pattern: "Summary: N aliases, N zones, N configs converted; N warnings"
func TestRun_StderrSummaryFormat(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:  "../../testdata/mds/basic.cfg",
		Direction:  "mds2brocade",
		FOSVersion: "pre-8.1",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)

	// Summary line must match the exact format
	summaryPattern := regexp.MustCompile(
		`Summary: \d+ aliases, \d+ zones, \d+ configs converted; \d+ warnings`,
	)
	require.Regexp(t, summaryPattern, stderr.String(),
		`stderr must contain a line matching "Summary: N aliases, N zones, N configs converted; N warnings"`)
}
