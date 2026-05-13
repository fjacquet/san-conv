package converter

import (
	"bytes"
	"io"
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

// --peer-consolidate: flat <init>_<target> zones collapse into per-target peer zones.
func TestRun_MDS2Brocade_Consolidate(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:   "../../testdata/mds/flat_zones.cfg",
		Direction:   "mds2brocade",
		Consolidate: true,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	// TGT1 has 4 hosts (incl. ESX9 via the relaxed <host>_HBA0_<target> name), TGT2 has 2 → two peer zones.
	require.Contains(t, out, `zonecreate --peerzone "TGT1_peerzone" -principal "TGT1" -members "ESX1;ESX2;ESX3;ESX9"`)
	require.Contains(t, out, `zonecreate --peerzone "TGT2_peerzone" -principal "TGT2" -members "ESX1;ESX2"`)
	// the original flat zones are gone, replaced — including the relaxed-matched one.
	require.NotContains(t, out, `zonecreate "ESX1_TGT1"`)
	require.NotContains(t, out, `zonecreate "ESX9_HBA0_TGT1"`)
	// the _SRDF zone (name doesn't decompose / target not a trailing component) and the 3-member zone stay flat.
	require.Contains(t, out, `zonecreate "RA1_RA2_SRDF", "RA1;RA2"`)
	require.Contains(t, out, `zonecreate "ThreeWay", "ESX1;ESX2;TGT1"`)
	// cfgcreate references the peer zones (deduped) and the un-consolidated zones.
	require.Regexp(t, regexp.MustCompile(`cfgcreate "ZS", "[^"]*TGT1_peerzone[^"]*TGT2_peerzone[^"]*RA1_RA2_SRDF[^"]*ThreeWay"`), out)
	// stderr has the summary line.
	require.Contains(t, stderr.String(), "Consolidated 6 flat zones into 2 peer zones; 1 zone(s) left flat")
}

// --consolidate-strict requires an exact <host>_<target> name — relaxed matches are left flat.
func TestRun_MDS2Brocade_ConsolidateStrict(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:         "../../testdata/mds/flat_zones.cfg",
		Direction:         "mds2brocade",
		Consolidate:       true,
		ConsolidateStrict: true,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	// ESX9_HBA0_TGT1 doesn't decompose to ESX9_TGT1 → left flat under --consolidate-strict.
	require.Contains(t, out, `zonecreate "ESX9_HBA0_TGT1", "ESX9;TGT1"`)
	// so TGT1_peerzone keeps only the strictly-matched hosts.
	require.Contains(t, out, `zonecreate --peerzone "TGT1_peerzone" -principal "TGT1" -members "ESX1;ESX2;ESX3"`)
	require.Contains(t, out, `zonecreate --peerzone "TGT2_peerzone" -principal "TGT2" -members "ESX1;ESX2"`)
	require.Contains(t, out, `zonecreate "RA1_RA2_SRDF", "RA1;RA2"`)
	require.Contains(t, stderr.String(), "Consolidated 5 flat zones into 2 peer zones; 2 zone(s) left flat")
}

// --consolidate-report writes a non-empty report file.
func TestRun_MDS2Brocade_ConsolidateReport(t *testing.T) {
	t.Parallel()
	reportFile := filepath.Join(t.TempDir(), "report.txt")
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:         "../../testdata/mds/flat_zones.cfg",
		Direction:         "mds2brocade",
		Consolidate:       true,
		ConsolidateReport: reportFile,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	content, readErr := os.ReadFile(reportFile)
	require.NoError(t, readErr)
	require.Contains(t, string(content), "Peer zones created")
	require.Contains(t, string(content), `peer zone "TGT1_peerzone"`)
	require.Contains(t, string(content), "RA1_RA2_SRDF")
}

// Without --peer-consolidate, output is the plain flat zones (no peer zones).
func TestRun_MDS2Brocade_NoConsolidateByDefault(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{InputFile: "../../testdata/mds/flat_zones.cfg", Direction: "mds2brocade"}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	require.NotContains(t, out, "--peerzone")
	require.Contains(t, out, `zonecreate "ESX1_TGT1", "ESX1;TGT1"`)
	require.NotContains(t, stderr.String(), "Consolidated")
}

// hygiene.Check runs on every conversion: a dangling alias reference is warned.
func TestRun_HygieneDanglingRef(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{InputFile: "../../testdata/mds/dangling_ref.cfg", Direction: "mds2brocade"}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	require.Contains(t, stderr.String(), `member alias "MissingTarget" is not defined — dangling reference`)
}

// TestRun_Brocade2MDS_PeerZone: a brocade CLI file with a --peerzone zone
// round-trips back to MDS smart zoning with the correct role keywords and
// zone smart-zoning enable vsan N line.
func TestRun_Brocade2MDS_PeerZone(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile: "../../testdata/brocade/peerzone_cli.cfg",
		Direction: "brocade2mds",
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)
	out := stdout.String()
	require.Contains(t, out, "zone smart-zoning enable vsan 1")
	require.Contains(t, out, "zone name TGT1_peerzone vsan 1")
	require.Contains(t, out, "  member device-alias TGT1 target")
	require.Contains(t, out, "  member device-alias ESX1 init")
}

// TestRun_RoundTrip_SmartZone: mds2brocade → file → brocade2mds → MDS smart zoning
// must produce correct smart-zoning enable and roled member lines.
// Note: the MDS `both` role maps to -principal in mds2brocade (connectivity-safe),
// then brocade2mds maps -principal back to `target` — so `both` becomes `target`
// after a round-trip. This is expected and documented in ADR-0010.
func TestRun_RoundTrip_SmartZone(t *testing.T) {
	t.Parallel()
	tmp := filepath.Join(t.TempDir(), "brocade.txt")
	var b1, b2 bytes.Buffer
	err := Run(Options{
		InputFile:  "../../testdata/mds/smart_zoning.cfg",
		Direction:  "mds2brocade",
		OutputFile: tmp,
	}, &b1, &b2)
	require.NoError(t, err, "mds2brocade must succeed")

	var out, errBuf bytes.Buffer
	err = Run(Options{
		InputFile: tmp,
		Direction: "brocade2mds",
	}, &out, &errBuf)
	require.NoError(t, err, "brocade2mds must succeed")

	result := out.String()
	require.Contains(t, result, "zone smart-zoning enable vsan 1")
	require.Contains(t, result, "zone name SmartZone vsan 1")
	// target member comes back as target
	require.Contains(t, result, "  member pwwn 50:06:0e:80:04:7c:00:01 target")
	// init member comes back as init
	require.Contains(t, result, "  member pwwn 50:05:0c:00:00:c8:aa:50 init")
	// SmartAliases zone
	require.Contains(t, result, "zone name SmartAliases vsan 1")
	require.Contains(t, result, "  member device-alias Array-Port target")
	require.Contains(t, result, "  member device-alias Host-A init")
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

func TestRun_Brocade2MDS_SmartConsolidate(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	err := Run(Options{
		InputFile:          "../../testdata/brocade/cli_flat_zones.cfg",
		Direction:          "brocade2mds",
		BrocadeConsolidate: true,
	}, &stdout, &stderr)
	require.NoError(t, err)

	out := stdout.String()
	// Smart-zoning must be enabled on the VSAN that holds the merged zones.
	require.Contains(t, out, "zone smart-zoning enable vsan 1",
		"missing smart-zoning enable directive")

	// TGT1 had 3 initiators (ESX1/ESX2/ESX3) → one merged zone named TGT1_smartzone.
	require.Contains(t, out, "zone name TGT1_smartzone vsan 1")
	require.Contains(t, out, "  member device-alias TGT1 target")
	require.Contains(t, out, "  member device-alias ESX1 init")
	require.Contains(t, out, "  member device-alias ESX2 init")
	require.Contains(t, out, "  member device-alias ESX3 init")

	// TGT2 had 2 initiators (ESX1/ESX2) → one merged zone named TGT2_smartzone.
	require.Contains(t, out, "zone name TGT2_smartzone vsan 1")
	require.Contains(t, out, "  member device-alias TGT2 target")

	// The SRDF replication zone stays flat: only DR_REPL_B is a trailing
	// component of the zone name, so the classifier infers target=DR_REPL_B;
	// but DR_REPL_B appears in only 1 candidate zone, so the frequency veto
	// kicks in (tgtFreq < 2).
	require.Contains(t, out, "zone name SRDF_DR_REPL_A_DR_REPL_B vsan 1",
		"single-occurrence zone should be left flat")
	require.NotContains(t, out, "DR_REPL_A_smartzone")
	require.NotContains(t, out, "DR_REPL_B_smartzone")

	// The 3-member zone is not consolidatable; it stays flat (and roleless).
	require.Contains(t, out, "zone name ThreeMemberZone vsan 1")

	// Original flat zones that WERE consolidated must be gone.
	require.NotContains(t, out, "zone name ESX1_TGT1 vsan")
	require.NotContains(t, out, "zone name ESX2_TGT2 vsan")

	// The zoneset must reference the merged zones, not the originals.
	require.Contains(t, out, "  member TGT1_smartzone")
	require.Contains(t, out, "  member TGT2_smartzone")
	require.NotContains(t, out, "  member ESX1_TGT1")

	// Summary line on stderr.
	require.Contains(t, stderr.String(),
		"Consolidated 5 flat zones into 2 smart zones; ")
}

// --consolidate-report writes a non-empty smart-zones report file when
// --smart-consolidate is on; vocabulary and direction-specific preamble
// must reflect the brocade2mds (smart-zone) direction.
func TestRun_Brocade2MDS_ConsolidateReport(t *testing.T) {
	t.Parallel()
	reportFile := filepath.Join(t.TempDir(), "report.txt")
	var stdout, stderr bytes.Buffer
	opts := Options{
		InputFile:          "../../testdata/brocade/cli_flat_zones.cfg",
		Direction:          "brocade2mds",
		BrocadeConsolidate: true,
		ConsolidateReport:  reportFile,
	}
	err := Run(opts, &stdout, &stderr)
	require.NoError(t, err)

	info, statErr := os.Stat(reportFile)
	require.NoError(t, statErr)
	require.Greater(t, info.Size(), int64(0), "report file must be non-empty")

	content, readErr := os.ReadFile(reportFile)
	require.NoError(t, readErr)
	s := string(content)

	// Heading + body vocabulary use the smart-zone words.
	require.Contains(t, s, "Smart zones consolidation report")
	require.Contains(t, s, `smart zone "TGT1_smartzone"`)
	// Preamble names the target-role member (MDS direction phrasing).
	require.Contains(t, s, "(the target-role member)")

	// Peer-direction vocabulary must NOT leak into the smart report.
	require.NotContains(t, s, `peer zone "`)
	require.NotContains(t, s, "(the -principal member)")
}

// --consolidate-strict in brocade2mds rejects trailing-component matches
// that are not exact <init>_<target>: relaxed mode merges them, strict
// mode leaves them flat.
func TestRun_Brocade2MDS_ConsolidateStrict(t *testing.T) {
	t.Parallel()

	// Relaxed (default): trailing-component classifier merges the two
	// three-component-name zones into TGT1_smartzone.
	var stdoutRelaxed, stderrRelaxed bytes.Buffer
	err := Run(Options{
		InputFile:          "../../testdata/brocade/cli_flat_zones_strict.cfg",
		Direction:          "brocade2mds",
		BrocadeConsolidate: true,
	}, &stdoutRelaxed, &stderrRelaxed)
	require.NoError(t, err)
	relaxed := stdoutRelaxed.String()
	require.Contains(t, relaxed, "zone smart-zoning enable vsan 1")
	require.Contains(t, relaxed, "zone name TGT1_smartzone vsan 1")
	require.Contains(t, relaxed, "  member device-alias TGT1 target")
	require.Contains(t, relaxed, "  member device-alias ESX1 init")
	require.Contains(t, relaxed, "  member device-alias ESX2 init")

	// Strict: zone names are not exact <init>_<target> form, so the
	// strict classifier leaves them flat — no smart zone is produced.
	var stdoutStrict, stderrStrict bytes.Buffer
	err = Run(Options{
		InputFile:          "../../testdata/brocade/cli_flat_zones_strict.cfg",
		Direction:          "brocade2mds",
		BrocadeConsolidate: true,
		ConsolidateStrict:  true,
	}, &stdoutStrict, &stderrStrict)
	require.NoError(t, err)
	strict := stdoutStrict.String()
	require.NotContains(t, strict, "TGT1_smartzone")
	require.Contains(t, strict, "zone name ESX1_HBA0_TGT1 vsan 1")
	require.Contains(t, strict, "zone name ESX2_HBA0_TGT1 vsan 1")
}

func TestRun_Brocade2MDS_NoSmartConsolidate_IsUnchanged(t *testing.T) {
	t.Parallel()
	var stdoutWith, stdoutWithout bytes.Buffer
	require.NoError(t, Run(Options{
		InputFile: "../../testdata/brocade/cli_flat_zones.cfg",
		Direction: "brocade2mds",
	}, &stdoutWithout, io.Discard))

	require.NoError(t, Run(Options{
		InputFile:          "../../testdata/brocade/cli_flat_zones.cfg",
		Direction:          "brocade2mds",
		BrocadeConsolidate: false, // explicit
	}, &stdoutWith, io.Discard))

	require.Equal(t, stdoutWithout.String(), stdoutWith.String(),
		"--smart-consolidate=false must produce byte-identical output")
	// No smart-zoning enable when no zones are roled.
	require.NotContains(t, stdoutWithout.String(), "smart-zoning enable")
}
