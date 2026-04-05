package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinto/ina/config"
	"github.com/spf13/cobra"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure Claude Code hooks and MCP server for ina integration",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		settingsPath := filepath.Join(home, ".claude", "settings.json")

		settings := make(map[string]interface{})
		if data, err := os.ReadFile(settingsPath); err == nil {
			if err := json.Unmarshal(data, &settings); err != nil {
				return fmt.Errorf("parse existing %s: %w", settingsPath, err)
			}
		}

		port := cfg.Daemon.GetHookPort()
		base := fmt.Sprintf("http://127.0.0.1:%d", port)

		// Build hooks config — each entry maps an event to an HTTP hook endpoint.
		hookEntry := func(matcher, path string, timeout int) []map[string]interface{} {
			return []map[string]interface{}{
				{"matcher": matcher, "hooks": []map[string]interface{}{
					{"type": "http", "url": base + path, "timeout": timeout},
				}},
			}
		}

		hooks := map[string]interface{}{
			"SessionStart": hookEntry("", "/hooks/session-start", 5),
			"SessionEnd":   hookEntry("", "/hooks/session-end", 5),
			"Stop":         hookEntry("", "/hooks/stop", 5),
			"PostToolUse":  hookEntry(".*", "/hooks/post-tool-use", 2),
		}

		// Merge hooks into existing settings
		existingHooks, _ := settings["hooks"].(map[string]interface{})
		if existingHooks == nil {
			existingHooks = make(map[string]interface{})
		}
		for k, v := range hooks {
			existingHooks[k] = v
		}
		settings["hooks"] = existingHooks

		// Find ina-mcp binary
		mcpPath := findInfaMCP()
		if mcpPath != "" {
			mcpServers, _ := settings["mcpServers"].(map[string]interface{})
			if mcpServers == nil {
				mcpServers = make(map[string]interface{})
			}
			mcpServers["ina"] = map[string]interface{}{
				"command": mcpPath,
			}
			settings["mcpServers"] = mcpServers
			fmt.Printf("MCP server: %s\n", mcpPath)
		}

		if err := os.MkdirAll(filepath.Dir(settingsPath), 0700); err != nil {
			return fmt.Errorf("create settings directory: %w", err)
		}
		data, err := json.MarshalIndent(settings, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(settingsPath, data, 0600); err != nil {
			return err
		}

		fmt.Printf("Hooks configured → %s\n", settingsPath)
		fmt.Printf("Hook endpoint: %s/hooks/*\n", base)
		fmt.Println("\nRun 'ina daemon' to start receiving events.")
		return nil
	},
}

func findInfaMCP() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	candidate := filepath.Join(filepath.Dir(exe), "ina-mcp")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	candidate = filepath.Join(config.DataDir(), "bin", "ina-mcp")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return ""
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
