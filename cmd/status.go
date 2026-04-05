package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/jinto/ina/agent"
	"github.com/jinto/ina/daemon"
	"github.com/spf13/cobra"
)

var statusJSON bool

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show all tracked agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := sendCommand(daemon.Command{Action: daemon.ActionStatus})
		if err != nil {
			return fmt.Errorf("daemon not running? %w", err)
		}

		var agents []agent.Snapshot
		if err := json.Unmarshal(resp.Data, &agents); err != nil {
			return err
		}

		if statusJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(agents)
		}

		if len(agents) == 0 {
			fmt.Println("No agents tracked.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATE\tAGENT\tPID\tLAST ACTIVE\tRESTARTS")
		for _, a := range agents {
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%d\n",
				a.Name, a.State, a.Kind, a.PID,
				timeSince(a.LastActive), a.RestartCount)
		}
		return w.Flush()
	},
}

func init() {
	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(statusCmd)
}

func sendCommand(cmd daemon.Command) (*daemon.Response, error) {
	return daemon.SendCommand(cmd)
}
