package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

		// Detect old HTTP hooks and notify user.
		if hasOldHTTPHooks(settings) {
			fmt.Println("Detected old HTTP hooks (port 9111) — replacing with resilient command hooks.")
		}

		// Build hooks config using command type for graceful degradation.
		inaPath := findIna()
		inaHooks := buildHooks(inaPath)

		// Merge: replace ina hooks, preserve all others.
		mergeInaHooks(settings, inaHooks)

		// Statusline — ina hud (ask user)
		if inaPath != "" {
			fmt.Println()
			fmt.Println("HUD statusline shows context usage and rate limits at the bottom of Claude Code.")
			fmt.Println("  Example: infinite-angel │ ██░░░ 38%  03:00 █░░░░  7d ░░░░░")
			if promptYesNo("Enable HUD?") {
				settings["statusLine"] = map[string]interface{}{
					"type":    "command",
					"command": inaPath + " hud",
				}
				fmt.Printf("Statusline: %s hud\n", inaPath)
				fmt.Println("  → To turn it off later: ina hud off")
			} else {
				fmt.Println("HUD skipped.")
				fmt.Println("  → To turn it on later: ina hud on")
			}
		}

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
		fmt.Printf("Hook command: %s hook <event>\n", inaPath)

		// Install Context7 MCP if not already configured
		setupContext7()

		// Detect Codex CLI
		setupCodex()

		// Install pre-push hook for LLM-Judge eval
		installPrePushHook()

		// Suggest install if LaunchAgent is not configured.
		if _, err := os.Stat(plistPath()); os.IsNotExist(err) {
			fmt.Println()
			if promptYesNo("Install ina as a launch agent (auto-start on login)?") {
				if err := installCmd.RunE(installCmd, nil); err != nil {
					fmt.Printf("Warning: install failed: %v\n", err)
				}
			} else {
				fmt.Println("Skipped. Run 'ina install' later to enable auto-start.")
			}
		} else {
			fmt.Println("\nLaunch agent already installed.")
		}
		return nil
	},
}

func findIna() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	real, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return exe
	}
	return real
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

func setupContext7() {
	// Check if context7 is already configured
	out, err := exec.Command("claude", "mcp", "list").CombinedOutput()
	if err == nil && strings.Contains(string(out), "context7") {
		fmt.Println("Context7 MCP: already configured")
		return
	}

	// Install Context7 (no API key required)
	fmt.Print("Installing Context7 MCP (library docs)... ")
	if err := exec.Command("claude", "mcp", "add", "context7", "--", "npx", "-y", "@upstash/context7-mcp").Run(); err != nil {
		fmt.Printf("skipped (%v)\n", err)
		return
	}
	fmt.Println("done")
}


func setupCodex() {
	path, err := exec.LookPath("codex")
	if err != nil {
		fmt.Println("Codex CLI: not found (install with: npm i -g @openai/codex)")
		return
	}
	out, err := exec.Command("codex", "--version").Output()
	if err != nil {
		fmt.Printf("Codex CLI: found at %s (version unknown)\n", path)
		return
	}
	fmt.Printf("Codex CLI: %s (%s)\n", strings.TrimSpace(string(out)), path)
}

func installPrePushHook() {
	// Find project root (git top-level)
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		fmt.Println("Pre-push hook: skipped (not a git repo)")
		return
	}
	root := strings.TrimSpace(string(out))

	src := filepath.Join(root, "scripts", "pre-push.sh")
	if _, err := os.Stat(src); err != nil {
		fmt.Println("Pre-push hook: skipped (scripts/pre-push.sh not found)")
		return
	}

	hooksDir := filepath.Join(root, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		fmt.Printf("Pre-push hook: skipped (%v)\n", err)
		return
	}

	dst := filepath.Join(hooksDir, "pre-push")

	// Backup existing hook if it's not ours
	if data, err := os.ReadFile(dst); err == nil {
		if !strings.Contains(string(data), "ina eval") {
			backup := dst + ".backup"
			os.WriteFile(backup, data, 0o755)
			fmt.Printf("Pre-push hook: backed up existing hook to %s\n", backup)
		}
	}

	// Copy hook script
	data, err := os.ReadFile(src)
	if err != nil {
		fmt.Printf("Pre-push hook: skipped (%v)\n", err)
		return
	}
	if err := os.WriteFile(dst, data, 0o755); err != nil {
		fmt.Printf("Pre-push hook: skipped (%v)\n", err)
		return
	}

	fmt.Printf("Pre-push hook: installed → %s\n", dst)
}

// promptYesNo prints a prompt with [Y/n] and returns true for yes/enter.
// Uses /dev/tty so it works even when stdin is a pipe (e.g. curl | sh).
func promptYesNo(prompt string) bool {
	fmt.Printf("%s [Y/n] ", prompt)
	var reader *bufio.Reader
	if tty, err := os.Open("/dev/tty"); err == nil {
		defer tty.Close()
		reader = bufio.NewReader(tty)
	} else {
		reader = bufio.NewReader(os.Stdin)
	}
	ans, _ := reader.ReadString('\n')
	ans = strings.TrimSpace(strings.ToLower(ans))
	return ans == "" || ans == "y" || ans == "yes"
}

// buildHooks creates the ina hook configuration using command type.
// Each hook runs `ina hook <event> 2>/dev/null || true` for graceful degradation.
func buildHooks(inaPath string) map[string]interface{} {
	hookEntry := func(matcher, event string, timeout int) []map[string]interface{} {
		cmd := fmt.Sprintf("%s hook %s 2>/dev/null || true", inaPath, event)
		return []map[string]interface{}{
			{"matcher": matcher, "hooks": []map[string]interface{}{
				{"type": "command", "command": cmd, "timeout": timeout},
			}},
		}
	}

	return map[string]interface{}{
		"SessionStart": hookEntry("", "session-start", 5),
		"SessionEnd":   hookEntry("", "session-end", 5),
		"Stop":         hookEntry("", "stop", 5),
		"PostToolUse":  hookEntry(".*", "post-tool-use", 2),
	}
}

// isInaHookEntry returns true if a hook entry belongs to ina
// (either old HTTP hook or new command hook).
func isInaHookEntry(entry interface{}) bool {
	raw, _ := json.Marshal(entry)
	s := string(raw)
	// Old HTTP hooks.
	if strings.Contains(s, "127.0.0.1") && strings.Contains(s, "/hooks/") {
		return true
	}
	// New command hooks.
	if strings.Contains(s, "ina hook") {
		return true
	}
	return false
}

// mergeInaHooks replaces ina hooks in settings while preserving all other hooks.
func mergeInaHooks(settings map[string]interface{}, inaHooks map[string]interface{}) {
	existingHooks, _ := settings["hooks"].(map[string]interface{})
	if existingHooks == nil {
		existingHooks = make(map[string]interface{})
	}

	for event, inaEntry := range inaHooks {
		inaArr, _ := inaEntry.([]map[string]interface{})

		existing, ok := existingHooks[event]
		if !ok {
			// No existing hooks for this event — just set ina's.
			existingHooks[event] = inaEntry
			continue
		}

		// Filter out old ina entries, keep everything else.
		var kept []interface{}
		switch arr := existing.(type) {
		case []interface{}:
			for _, entry := range arr {
				if !isInaHookEntry(entry) {
					kept = append(kept, entry)
				}
			}
		case []map[string]interface{}:
			for _, entry := range arr {
				if !isInaHookEntry(entry) {
					kept = append(kept, entry)
				}
			}
		}

		// Append ina's new entry.
		for _, entry := range inaArr {
			kept = append(kept, entry)
		}
		existingHooks[event] = kept
	}

	settings["hooks"] = existingHooks
}

// hasOldHTTPHooks detects old ina HTTP hooks (not command hooks) in settings.
func hasOldHTTPHooks(settings map[string]interface{}) bool {
	hooks, _ := settings["hooks"].(map[string]interface{})
	if hooks == nil {
		return false
	}
	for _, event := range []string{"SessionStart", "SessionEnd", "Stop", "PostToolUse"} {
		entries, ok := hooks[event]
		if !ok {
			continue
		}
		raw, _ := json.Marshal(entries)
		s := string(raw)
		// Only flag HTTP hooks, not already-converted command hooks.
		if strings.Contains(s, "127.0.0.1") && strings.Contains(s, "/hooks/") && strings.Contains(s, "\"http\"") {
			return true
		}
	}
	return false
}

func init() {
	rootCmd.AddCommand(setupCmd)
}
