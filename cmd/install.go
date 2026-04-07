package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

var purgeFlag bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove ina daemon, hooks, HUD, and MCP server from Claude Code",
	Long:  "Remove all ina integrations from Claude Code.\nUse --purge to also delete ~/.ina (config, logs, registry).",
	RunE: func(cmd *cobra.Command, args []string) error {
		// 1. LaunchAgent
		path := plistPath()
		exec.Command("launchctl", "unload", path).Run()
		if err := os.Remove(path); err == nil {
			fmt.Printf("Removed: %s\n", path)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("remove plist: %w", err)
		}

		// 2. Clean settings.json (hooks, statusLine, mcpServers.ina)
		if err := cleanSettings(); err != nil {
			fmt.Printf("Warning: settings cleanup failed: %v\n", err)
		}

		// 3. Restore pre-push hook
		restorePrePushHook()

		// 4. Purge data directory
		if purgeFlag {
			dataDir := config.DataDir()
			if err := os.RemoveAll(dataDir); err != nil {
				fmt.Printf("Warning: failed to remove %s: %v\n", dataDir, err)
			} else {
				fmt.Printf("Removed: %s\n", dataDir)
			}
		}

		fmt.Println("\nina has been uninstalled.")
		return nil
	},
}

func cleanSettings() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return fmt.Errorf("parse %s: %w", settingsPath, err)
	}

	changed := false

	// Remove statusLine if it points to ina
	if sl, ok := settings["statusLine"].(map[string]interface{}); ok {
		if cmd, _ := sl["command"].(string); strings.Contains(cmd, "ina hud") {
			delete(settings, "statusLine")
			changed = true
			fmt.Println("Removed: statusLine (HUD)")
		}
	}

	// Remove ina hooks (entries pointing to /hooks/ endpoints)
	if hooks, ok := settings["hooks"].(map[string]interface{}); ok {
		var removed []string
		for _, event := range []string{"SessionStart", "SessionEnd", "Stop", "PostToolUse"} {
			if entry, exists := hooks[event]; exists {
				raw, _ := json.Marshal(entry)
				if strings.Contains(string(raw), "127.0.0.1") && strings.Contains(string(raw), "/hooks/") {
					delete(hooks, event)
					removed = append(removed, event)
					changed = true
				}
			}
		}
		if len(hooks) == 0 {
			delete(settings, "hooks")
		}
		if len(removed) > 0 {
			fmt.Printf("Removed: hooks (%s)\n", strings.Join(removed, ", "))
		}
	}

	// Remove mcpServers.ina
	if mcp, ok := settings["mcpServers"].(map[string]interface{}); ok {
		if _, exists := mcp["ina"]; exists {
			delete(mcp, "ina")
			changed = true
			fmt.Println("Removed: mcpServers.ina")
		}
		if len(mcp) == 0 {
			delete(settings, "mcpServers")
		}
	}

	if !changed {
		fmt.Println("settings.json: nothing to clean")
		return nil
	}

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath, out, 0600); err != nil {
		return err
	}
	fmt.Printf("Updated: %s\n", settingsPath)
	return nil
}

func restorePrePushHook() {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return
	}
	hookPath := filepath.Join(strings.TrimSpace(string(out)), ".git", "hooks", "pre-push")

	data, err := os.ReadFile(hookPath)
	if err != nil {
		return
	}
	if !strings.Contains(string(data), "ina eval") {
		return
	}

	backup := hookPath + ".backup"
	if _, err := os.Stat(backup); err == nil {
		_ = os.Rename(backup, hookPath)
		fmt.Println("Restored: pre-push hook from backup")
	} else {
		os.Remove(hookPath)
		fmt.Println("Removed: pre-push hook")
	}
}

func init() {
	rootCmd.AddCommand(installCmd)
	uninstallCmd.Flags().BoolVar(&purgeFlag, "purge", false, "also remove ~/.ina (config, logs, registry)")
	rootCmd.AddCommand(uninstallCmd)
}
