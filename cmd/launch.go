package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/jinto/ina/daemon"
	"github.com/spf13/cobra"
)

var (
	launchAgent   string
	launchName    string
	launchWorktree bool
)

var launchCmd = &cobra.Command{
	Use:   "launch <path> <task>",
	Short: "Launch a new agent on a project",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"path":     absPath,
			"task":     args[1],
			"agent":    launchAgent,
			"name":     launchName,
			"worktree": launchWorktree,
		})

		resp, err := sendCommand(daemon.Command{
			Action: daemon.ActionLaunch,
			Data:   payload,
		})
		if err != nil {
			return err
		}

		fmt.Printf("Agent launched: %s\n", resp.Message)
		return nil
	},
}

func init() {
	launchCmd.Flags().StringVar(&launchAgent, "agent", "", "Agent type: claude or codex")
	launchCmd.Flags().StringVar(&launchName, "name", "", "Human-readable name for this agent")
	launchCmd.Flags().BoolVar(&launchWorktree, "worktree", false, "Create git worktree for isolation")
	rootCmd.AddCommand(launchCmd)
}
