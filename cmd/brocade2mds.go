package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var brocade2mdsCmd = &cobra.Command{
	Use:   "brocade2mds [input-file]",
	Short: "Convert Brocade FOS cfgshow or CLI script to Cisco MDS NX-OS commands",
	Long: `brocade2mds parses a Brocade FOS cfgshow output or CLI script and produces
NX-OS CLI commands (device-alias database, zone, zoneset, zoneset activate).`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("brocade2mds: not yet implemented")
	},
}

func init() {
	brocade2mdsCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
}
