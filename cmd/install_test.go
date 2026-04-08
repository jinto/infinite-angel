package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanSettings_RemovesInaCommandHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				// Other tool hook — must survive.
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "other-tool notify",
						},
					},
				},
				// ina command hook — must be removed.
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "/usr/local/bin/ina hook post-tool-use 2>/dev/null || true",
						},
					},
				},
			},
			"SessionStart": []interface{}{
				map[string]interface{}{
					"matcher": "",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "/usr/local/bin/ina hook session-start 2>/dev/null || true",
						},
					},
				},
			},
		},
	}

	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, data, 0600)

	cleanSettingsFile(settingsPath)

	result, _ := os.ReadFile(settingsPath)
	var out map[string]interface{}
	json.Unmarshal(result, &out)

	hooks, _ := out["hooks"].(map[string]interface{})

	// PostToolUse should still have 1 entry (other-tool).
	pt, _ := hooks["PostToolUse"].([]interface{})
	if len(pt) != 1 {
		t.Fatalf("PostToolUse should have 1 entry, got %d", len(pt))
	}
	raw, _ := json.Marshal(pt[0])
	if !strings.Contains(string(raw), "other-tool") {
		t.Errorf("surviving hook should be other-tool: %s", raw)
	}

	// SessionStart should be removed entirely (was ina-only).
	if _, exists := hooks["SessionStart"]; exists {
		t.Error("SessionStart should be removed (was ina-only)")
	}
}

func TestCleanSettings_RemovesOldHTTPHooks(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "settings.json")

	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type": "http",
							"url":  "http://127.0.0.1:9111/hooks/post-tool-use",
						},
					},
				},
			},
		},
	}

	data, _ := json.MarshalIndent(settings, "", "  ")
	os.WriteFile(settingsPath, data, 0600)

	cleanSettingsFile(settingsPath)

	result, _ := os.ReadFile(settingsPath)
	var out map[string]interface{}
	json.Unmarshal(result, &out)

	hooks, _ := out["hooks"].(map[string]interface{})
	if _, exists := hooks["PostToolUse"]; exists {
		t.Error("old HTTP hook should be removed")
	}
}

func init() {
	_ = os.MkdirAll(filepath.Join(os.TempDir(), ".ina"), 0700)
}
