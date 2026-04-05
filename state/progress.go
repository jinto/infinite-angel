package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Progress struct {
	Task         string `yaml:"task"`
	Agent        string `yaml:"agent"`
	SessionID    string `yaml:"session_id"`
	UpdatedAt    string `yaml:"updated_at"`
	Status       string `yaml:"status"`
	Blocked      bool   `yaml:"blocked"`
	RestartCount int    `yaml:"restart_count"`

	// Parsed markdown sections
	Completed  string `yaml:"-"`
	InProgress string `yaml:"-"`
	Remaining  string `yaml:"-"`
	Context    string `yaml:"-"`
}

func stateDir(projectDir string) string {
	return filepath.Join(projectDir, ".state")
}

func progressPath(projectDir string) string {
	return filepath.Join(stateDir(projectDir), "progress.md")
}

// Init creates the initial state file for a new agent session.
func Init(projectDir, task, agentKind string) error {
	dir := stateDir(projectDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	content := fmt.Sprintf(`---
task: %q
agent: %q
session_id: ""
updated_at: %q
status: "pending"
blocked: false
restart_count: 0
---

## Completed

## In Progress

## Remaining
- [ ] %s

## Context for Restart
Starting fresh.
`, task, agentKind, time.Now().Format(time.RFC3339), task)

	return os.WriteFile(progressPath(projectDir), []byte(content), 0600)
}

// Read parses the state file from a project directory.
func Read(projectDir string) (*Progress, error) {
	data, err := os.ReadFile(progressPath(projectDir))
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

// Parse extracts YAML frontmatter and markdown sections from state file content.
func Parse(content string) (*Progress, error) {
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid state file: missing frontmatter")
	}

	p := &Progress{}
	if err := yaml.Unmarshal([]byte(parts[1]), p); err != nil {
		return nil, fmt.Errorf("parse frontmatter: %w", err)
	}

	body := parts[2]
	p.Completed = extractSection(body, "## Completed")
	p.InProgress = extractSection(body, "## In Progress")
	p.Remaining = extractSection(body, "## Remaining")
	p.Context = extractSection(body, "## Context for Restart")

	return p, nil
}

func extractSection(body, header string) string {
	idx := strings.Index(body, header)
	if idx == -1 {
		return ""
	}

	rest := body[idx+len(header):]

	// Find next section header
	nextIdx := strings.Index(rest, "\n## ")
	if nextIdx == -1 {
		return strings.TrimSpace(rest)
	}

	return strings.TrimSpace(rest[:nextIdx])
}
