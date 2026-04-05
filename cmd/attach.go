package cmd

import (
	"github.com/spf13/cobra"
)

var attachCmd = &cobra.Command{
	Use:   "attach <name>",
	Short: "Attach to an agent's live log output",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		logPath, err := findLatestLog(args[0])
		if err != nil {
			return err
		}
		return tailFile(logPath, true)
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)
}
