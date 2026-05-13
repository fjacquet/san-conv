package cmd

import (
	"os"

	"github.com/fjacquet/san-conv/internal/converter"
	"github.com/spf13/cobra"
)

var brocade2mdsCmd = &cobra.Command{
	Use:   "brocade2mds [input-file]",
	Short: "Convert Brocade FOS cfgshow or CLI script to Cisco MDS NX-OS commands",
	Long: `brocade2mds parses a Brocade FOS cfgshow output or CLI script and
produces NX-OS CLI commands (device-alias database, zone, zoneset,
zoneset activate).

With --smart-consolidate, flat single-initiator/single-target Brocade
zones are collapsed into per-target MDS smart zones (target as principal,
hosts as init members), and "zone smart-zoning enable vsan N" is emitted
automatically. The target is inferred from the zone name (default: the
target alias is a trailing component of the zone name; --consolidate-strict
requires exact <host>_<target>).
ALWAYS review --consolidate-report before applying.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile, _ := cmd.Flags().GetString("output")
		consolidate, _ := cmd.Flags().GetBool("smart-consolidate")
		consolidateReport, _ := cmd.Flags().GetString("consolidate-report")
		consolidateStrict, _ := cmd.Flags().GetBool("consolidate-strict")

		return converter.Run(converter.Options{
			InputFile:          args[0],
			Direction:          "brocade2mds",
			OutputFile:         outputFile,
			BrocadeConsolidate: consolidate,
			ConsolidateReport:  consolidateReport,
			ConsolidateStrict:  consolidateStrict,
		}, os.Stdout, os.Stderr)
	},
}

func init() {
	brocade2mdsCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
	brocade2mdsCmd.Flags().Bool("smart-consolidate", false, "consolidate flat single-initiator/single-target zones into per-target MDS smart zones (inferred — review with --consolidate-report)")
	brocade2mdsCmd.Flags().String("consolidate-report", "", "write the consolidation report to this file")
	brocade2mdsCmd.Flags().Bool("consolidate-strict", false, "with --smart-consolidate: require an exact <host>_<target> zone name (default: also consolidate when the target alias is a trailing component of the zone name)")
}
