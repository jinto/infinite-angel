package state

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	content := `---
task: "Build auth system"
agent: "claude"
session_id: "abc-123"
updated_at: "2026-04-04T01:00:00Z"
status: "in_progress"
blocked: false
restart_count: 1
---

## Completed
- [x] DB schema created

## In Progress
- [ ] OAuth integration

## Remaining
- [ ] Write tests

## Context for Restart
allauth configured, next add social providers.
`
	p, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if p.Task != "Build auth system" {
		t.Errorf("Task = %q, want %q", p.Task, "Build auth system")
	}
	if p.Agent != "claude" {
		t.Errorf("Agent = %q, want %q", p.Agent, "claude")
	}
	if p.Blocked {
		t.Error("Blocked = true, want false")
	}
	if p.RestartCount != 1 {
		t.Errorf("RestartCount = %d, want 1", p.RestartCount)
	}
	if p.Completed == "" {
		t.Error("Completed section is empty")
	}
	if p.InProgress == "" {
		t.Error("InProgress section is empty")
	}
	if p.Context == "" {
		t.Error("Context section is empty")
	}
}

func TestParseMissingFrontmatter(t *testing.T) {
	_, err := Parse("no frontmatter here")
	if err == nil {
		t.Error("expected error for missing frontmatter")
	}
}

func TestParseBlockedTrue(t *testing.T) {
	content := `---
task: "blocked task"
blocked: true
---

## Completed
`
	p, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !p.Blocked {
		t.Error("Blocked = false, want true")
	}
}

func TestInitAndRead(t *testing.T) {
	dir := t.TempDir()

	if err := Init(dir, "test task", "claude"); err != nil {
		t.Fatalf("Init: %v", err)
	}

	statePath := filepath.Join(dir, ".state", "progress.md")
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	p, err := Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if p.Task != "test task" {
		t.Errorf("Task = %q, want %q", p.Task, "test task")
	}
	if p.Agent != "claude" {
		t.Errorf("Agent = %q, want %q", p.Agent, "claude")
	}
	if p.Status != "pending" {
		t.Errorf("Status = %q, want %q", p.Status, "pending")
	}
}
