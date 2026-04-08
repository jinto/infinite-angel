package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildHooks_CommandType(t *testing.T) {
	hooks := buildHooks("/usr/local/bin/ina")
	for event, entries := range hooks {
		arr, ok := entries.([]map[string]interface{})
		if !ok || len(arr) == 0 {
			t.Fatalf("%s: expected non-empty array", event)
		}
		hookList, ok := arr[0]["hooks"].([]map[string]interface{})
		if !ok || len(hookList) == 0 {
			t.Fatalf("%s: missing hooks array", event)
		}
		h := hookList[0]
		if h["type"] != "command" {
			t.Errorf("%s: type = %v, want command", event, h["type"])
		}
		cmd, _ := h["command"].(string)
		if cmd == "" {
			t.Errorf("%s: command is empty", event)
		}
		// Must contain || true for exit 127 protection.
		if !containsStr(cmd, "|| true") {
			t.Errorf("%s: command %q missing '|| true'", event, cmd)
		}
		// Must contain the ina binary path.
		if !containsStr(cmd, "/usr/local/bin/ina") {
			t.Errorf("%s: command %q missing ina path", event, cmd)
		}
	}
}

func TestMergeHooks_PreservesOtherHooks(t *testing.T) {
	// Existing settings with a non-ina PostToolUse hook.
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "other-tool notify",
						},
					},
				},
			},
		},
	}

	inaHooks := buildHooks("/usr/local/bin/ina")
	mergeInaHooks(existing, inaHooks)

	// PostToolUse should now have 2 entries: other-tool + ina.
	hooks := existing["hooks"].(map[string]interface{})
	ptEntries, ok := hooks["PostToolUse"].([]interface{})
	if !ok {
		t.Fatalf("PostToolUse should be []interface{}, got %T", hooks["PostToolUse"])
	}
	if len(ptEntries) != 2 {
		t.Fatalf("PostToolUse should have 2 entries (other + ina), got %d", len(ptEntries))
	}

	// First entry should be the other tool's hook (preserved).
	first, _ := json.Marshal(ptEntries[0])
	if !containsStr(string(first), "other-tool") {
		t.Errorf("first entry should be other-tool: %s", first)
	}
}

func TestMergeHooks_ReplacesOldHTTPHook(t *testing.T) {
	// Existing settings with old ina HTTP hook.
	existing := map[string]interface{}{
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

	inaHooks := buildHooks("/usr/local/bin/ina")
	mergeInaHooks(existing, inaHooks)

	hooks := existing["hooks"].(map[string]interface{})
	ptEntries, _ := hooks["PostToolUse"].([]interface{})
	// Old HTTP hook should be replaced, not duplicated.
	if len(ptEntries) != 1 {
		t.Fatalf("PostToolUse should have 1 entry (old replaced), got %d", len(ptEntries))
	}
	raw, _ := json.Marshal(ptEntries[0])
	if containsStr(string(raw), "127.0.0.1:9111") {
		t.Errorf("old HTTP hook should be replaced: %s", raw)
	}
	if !containsStr(string(raw), "ina hook") {
		t.Errorf("new command hook should contain 'ina hook': %s", raw)
	}
}

func TestMergeHooks_ReplacesOldInaCommandHook(t *testing.T) {
	// Idempotency: running setup twice should not duplicate ina hooks.
	existing := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"matcher": ".*",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "/old/path/ina hook post-tool-use 2>/dev/null || true",
						},
					},
				},
			},
		},
	}

	inaHooks := buildHooks("/new/path/ina")
	mergeInaHooks(existing, inaHooks)

	hooks := existing["hooks"].(map[string]interface{})
	ptEntries, _ := hooks["PostToolUse"].([]interface{})
	if len(ptEntries) != 1 {
		t.Fatalf("should have 1 entry after idempotent merge, got %d", len(ptEntries))
	}
	raw, _ := json.Marshal(ptEntries[0])
	if !containsStr(string(raw), "/new/path/ina") {
		t.Errorf("should use new path: %s", raw)
	}
}

func TestDetectOldHTTPHooks(t *testing.T) {
	settings := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
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
	if !hasOldHTTPHooks(settings) {
		t.Error("should detect old HTTP hooks")
	}

	settings2 := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PostToolUse": []interface{}{
				map[string]interface{}{
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": "ina hook post-tool-use || true",
						},
					},
				},
			},
		},
	}
	if hasOldHTTPHooks(settings2) {
		t.Error("should not flag command hooks as old HTTP hooks")
	}
}

func containsStr(s, substr string) bool {
	return strings.Contains(s, substr)
}

func init() {
	// Ensure test doesn't fail on missing config dir.
	_ = os.MkdirAll(filepath.Join(os.TempDir(), ".ina"), 0700)
}
