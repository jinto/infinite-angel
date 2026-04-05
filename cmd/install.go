package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/jinto/ina/config"
	"github.com/spf13/cobra"
)

const plistLabel = "com.jinto.ina"

var plistTmpl = template.Must(template.New("plist").Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>{{.Label}}</string>
	<key>ProgramArguments</key>
	<array>
		<string>{{.Binary}}</string>
		<string>daemon</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>ThrottleInterval</key>
	<integer>10</integer>
	<key>StandardOutPath</key>
	<string>{{.LogPath}}</string>
	<key>StandardErrorPath</key>
	<string>{{.LogPath}}</string>
</dict>
</plist>
`))

type plistData struct {
	Label   string
	Binary  string
	LogPath string
}

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", plistLabel+".plist")
}

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install ina daemon as a macOS launch agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		binPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve binary path: %w", err)
		}
		binPath, err = filepath.EvalSymlinks(binPath)
		if err != nil {
			return fmt.Errorf("resolve symlinks: %w", err)
		}

		home, _ := os.UserHomeDir()
		launchAgentDir := filepath.Join(home, "Library", "LaunchAgents")
		if err := os.MkdirAll(launchAgentDir, 0755); err != nil {
			return fmt.Errorf("create LaunchAgents dir: %w", err)
		}

		path := plistPath()
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("create plist: %w", err)
		}

		data := plistData{
			Label:   plistLabel,
			Binary:  binPath,
			LogPath: filepath.Join(config.DataDir(), "launchd.log"),
		}
		if err := plistTmpl.Execute(f, data); err != nil {
			f.Close()
			return fmt.Errorf("write plist: %w", err)
		}
		f.Close()

		out, err := exec.Command("launchctl", "load", path).CombinedOutput()
		if err != nil {
			return fmt.Errorf("launchctl load: %s: %w", out, err)
		}

		fmt.Printf("Installed: %s\n", path)
		fmt.Printf("Binary:    %s\n", binPath)
		fmt.Println("Daemon will start automatically on login.")
		fmt.Println("Use 'ina uninstall' to remove.")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove ina daemon launch agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := plistPath()
		exec.Command("launchctl", "unload", path).Run()

		if err := os.Remove(path); os.IsNotExist(err) {
			fmt.Println("Not installed.")
			return nil
		} else if err != nil {
			return fmt.Errorf("remove plist: %w", err)
		}

		fmt.Printf("Removed: %s\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(uninstallCmd)
}
