package cmd

import (
	"os"

	"github.com/fjacquet/san-conv/internal/converter"
	"github.com/spf13/cobra"
)

var brocade2mdsCmd = &cobra.Command{
	Use:   "brocade2mds [input-file]",
	Short: "Convert Brocade FOS cfgshow or CLI script to Cisco MDS NX-OS commands",
	Long: `brocade2mds parses a Brocade FOS cfgshow output or CLI script and produces
NX-OS CLI commands (device-alias database, zone, zoneset, zoneset activate).`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		outputFile, _ := cmd.Flags().GetString("output")

		return converter.Run(converter.Options{
			InputFile:  args[0],
			Direction:  "brocade2mds",
			OutputFile: outputFile,
		}, os.Stdout, os.Stderr)
	},
}

func init() {
	brocade2mdsCmd.Flags().String("output", "", "write primary output to file (default: stdout)")
}
