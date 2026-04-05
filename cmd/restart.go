package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/jinto/ina/daemon"
	"github.com/spf13/cobra"
)

var restartFresh bool

var restartCmd = &cobra.Command{
	Use:   "restart <name|id>",
	Short: "Restart a dead or stalled agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload, _ := json.Marshal(map[string]interface{}{
			"target": args[0],
			"fresh":  restartFresh,
		})

		resp, err := sendCommand(daemon.Command{
			Action: daemon.ActionRestart,
			Data:   payload,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Agent restarted: %s\n", resp.Message)
		return nil
	},
}

func init() {
	restartCmd.Flags().BoolVar(&restartFresh, "fresh", false, "Don't inject previous state")
	rootCmd.AddCommand(restartCmd)
}
