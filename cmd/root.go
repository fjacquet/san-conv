package cmd

import (
	"os"

	"github.com/fjacquet/san-conv/internal/converter"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "san-conv",
	Short: "Convert SAN zoning configurations between Cisco MDS and Brocade FOS formats",
	Long: `san-conv converts SAN fabric zoning configurations between
Cisco MDS NX-OS and Brocade FOS formats.

Primary use case: mds2brocade (MDS running-config to FOS CLI commands)
Reverse direction: brocade2mds (FOS cfgshow/script to NX-OS CLI commands)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		direction, _ := cmd.Flags().GetString("direction")
		outputFile, _ := cmd.Flags().GetString("output")
		scriptFile, _ := cmd.Flags().GetString("script")
		fosVersion, _ := cmd.Flags().GetString("fos-version")
		return converter.Run(converter.Options{
			InputFile:  args[0],
			Direction:  direction,
			OutputFile: outputFile,
			ScriptFile: scriptFile,
			FOSVersion: fosVersion,
		}, os.Stdout, os.Stderr)
	},
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

	// Root-level flags — activated when no subcommand is given (CLI-01, CLI-02)
	rootCmd.Flags().StringP("direction", "d", "mds2brocade", "conversion direction: mds2brocade or brocade2mds")
	rootCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
	rootCmd.Flags().String("script", "", "also write executable shell script to file (mds2brocade only)")
	rootCmd.Flags().String("fos-version", "pre-8.1", "target FOS naming rules (pre-8.1 or 8.1+)")
}
