package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/jinto/ina/daemon"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop <name|id>",
	Short: "Stop a specific agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		payload, _ := json.Marshal(map[string]string{
			"target": args[0],
		})

		resp, err := sendCommand(daemon.Command{
			Action: daemon.ActionStop,
			Data:   payload,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Agent stopped: %s\n", resp.Message)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stopCmd)
}
