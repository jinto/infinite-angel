package cmd

import (
	"github.com/spf13/cobra"
)

var attachRaw bool

var attachCmd = &cobra.Command{
	Use:   "attach <name>",
	Short: "Attach to an agent's live log output",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logPath, err := findLatestLog(args[0])
		if err != nil {
			return err
		}
		if attachRaw {
			return tailFile(logPath, true)
		}
		return prettyTail(logPath, 50, true)
	},
}

func init() {
	attachCmd.Flags().BoolVar(&attachRaw, "raw", false, "Show raw JSON output")
	rootCmd.AddCommand(attachCmd)
}
