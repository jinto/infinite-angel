package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jinto/ina/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the watchdog daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		d, err := daemon.New(cfg)
		if err != nil {
			return err
		}

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		errCh := make(chan error, 1)
		go func() { errCh <- d.Run() }()

		fmt.Println("ina daemon started")

		// Check for updates now and every 24 hours
		go CheckForUpdate()
		updateTicker := time.NewTicker(24 * time.Hour)
		defer updateTicker.Stop()

		for {
			select {
			case sig := <-sigCh:
				fmt.Printf("\nreceived %s, shutting down...\n", sig)
				d.Stop()
				return <-errCh
			case err := <-errCh:
				return err
			case <-updateTicker.C:
				go CheckForUpdate()
			}
		}
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		return daemon.StopRunning()
	},
}

func init() {
	daemonCmd.AddCommand(daemonStopCmd)
	rootCmd.AddCommand(daemonCmd)
}
