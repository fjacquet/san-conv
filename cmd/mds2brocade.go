package cmd

import (
	"os"

	"github.com/fjacquet/san-conv/internal/converter"
	"github.com/spf13/cobra"
)

var mds2brocadeCmd = &cobra.Command{
	Use:   "mds2brocade [input-file]",
	Short: "Convert Cisco MDS NX-OS running-config to Brocade FOS CLI commands",
	Long: `mds2brocade parses a Cisco MDS NX-OS running-config file and produces
ready-to-apply Brocade FOS CLI commands (alicreate, zonecreate, cfgcreate).

The output includes a defzone --noaccess preamble and cfgsave postamble.
cfgenable is present as a commented-out line requiring human confirmation.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile, _ := cmd.Flags().GetString("output")
		scriptFile, _ := cmd.Flags().GetString("script")
		fosVersion, _ := cmd.Flags().GetString("fos-version")
		vsan, _ := cmd.Flags().GetInt("vsan")
		consolidate, _ := cmd.Flags().GetBool("peer-consolidate")
		consolidateReport, _ := cmd.Flags().GetString("consolidate-report")

		return converter.Run(converter.Options{
			InputFile:         args[0],
			Direction:         "mds2brocade",
			OutputFile:        outputFile,
			ScriptFile:        scriptFile,
			FOSVersion:        fosVersion,
			VSAN:              vsan,
			Consolidate:       consolidate,
			ConsolidateReport: consolidateReport,
		}, os.Stdout, os.Stderr)
	},
}

func init() {
	// Flags will be added in Phase 7 (CLI Wiring)
	// Stub them here as empty declarations so --help shows them
	mds2brocadeCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
	mds2brocadeCmd.Flags().String("script", "", "also write executable shell script to file")
	mds2brocadeCmd.Flags().String("fos-version", "8.1+", "target FOS naming rules (pre-8.1 or 8.1+)")
	mds2brocadeCmd.Flags().Int("vsan", 0, "target VSAN to convert; 0 = convert all VSANs into one fabric")
	mds2brocadeCmd.Flags().Bool("peer-consolidate", false, "consolidate flat single-initiator/single-target zones into per-target Brocade peer zones (inferred — review with --consolidate-report)")
	mds2brocadeCmd.Flags().String("consolidate-report", "", "write the peer-zone consolidation report to this file")
}
