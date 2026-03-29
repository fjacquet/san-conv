package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var mds2brocadeCmd = &cobra.Command{
	Use:   "mds2brocade [input-file]",
	Short: "Convert Cisco MDS NX-OS running-config to Brocade FOS CLI commands",
	Long: `mds2brocade parses a Cisco MDS NX-OS running-config file and produces
ready-to-apply Brocade FOS CLI commands (alicreate, zonecreate, cfgcreate).

The output includes a defzone --noaccess preamble and cfgsave postamble.
cfgenable is present as a commented-out line requiring human confirmation.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("mds2brocade: not yet implemented")
	},
}

func init() {
	// Flags will be added in Phase 7 (CLI Wiring)
	// Stub them here as empty declarations so --help shows them
	mds2brocadeCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
	mds2brocadeCmd.Flags().String("script", "", "also write executable shell script to file")
	mds2brocadeCmd.Flags().String("fos-version", "pre-8.1", "target FOS naming rules (pre-8.1 or 8.1+)")
}
