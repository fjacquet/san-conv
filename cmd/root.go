package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "san-conv",
	Short: "Convert SAN zoning configurations between Cisco MDS and Brocade FOS formats",
	Long: `san-conv converts SAN fabric zoning configurations between
Cisco MDS NX-OS and Brocade FOS formats.

Primary use case: mds2brocade (MDS running-config → FOS CLI commands)
Reverse direction: brocade2mds (FOS cfgshow/script → NX-OS CLI commands)`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SilenceUsage = true // suppress usage on runtime errors (not flag errors)
	rootCmd.AddCommand(mds2brocadeCmd)
	rootCmd.AddCommand(brocade2mdsCmd)
}
