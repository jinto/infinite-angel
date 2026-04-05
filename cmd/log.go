package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jinto/ina/config"
	"github.com/spf13/cobra"
)

var logFollow bool

var logCmd = &cobra.Command{
	Use:   "log [agent-name]",
	Short: "View agent or daemon logs",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var logPath string

		if len(args) == 0 {
			logPath = config.LogFile()
		} else {
			var err error
			logPath, err = findLatestLog(args[0])
			if err != nil {
				return err
			}
		}

		if _, err := os.Stat(logPath); err != nil {
			return fmt.Errorf("log file not found: %s", logPath)
		}

		return tailFile(logPath, logFollow)
	},
}

func findLatestLog(agentName string) (string, error) {
	logDir := filepath.Join(config.DataDir(), "logs", agentName)

	entries, err := os.ReadDir(logDir)
	if err != nil {
		return "", fmt.Errorf("no logs for agent %q", agentName)
	}

	var latest string
	for _, e := range entries {
		if !e.IsDir() && e.Name() > latest {
			latest = e.Name()
		}
	}

	if latest == "" {
		return "", fmt.Errorf("no logs for agent %q", agentName)
	}

	return filepath.Join(logDir, latest), nil
}

func tailFile(path string, follow bool) error {
	args := []string{"-50", path}
	if follow {
		args = []string{"-50f", path}
	}

	cmd := exec.Command("tail", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func init() {
	logCmd.Flags().BoolVarP(&logFollow, "follow", "f", false, "Follow log output")
	rootCmd.AddCommand(logCmd)
}
