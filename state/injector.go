package state

import (
	"fmt"
	"os/exec"
	"strings"
)

// BuildRestartPrompt composes a continuation prompt from state file + git history.
func BuildRestartPrompt(projectDir string) (string, error) {
	progress, err := Read(projectDir)
	if err != nil {
		return "", fmt.Errorf("read state: %w", err)
	}

	gitLog := recentGitLog(projectDir)

	var b strings.Builder

	b.WriteString("You are continuing a task that was interrupted. Here is the context:\n\n")

	b.WriteString("## Task\n")
	b.WriteString(progress.Task)
	b.WriteString("\n\n")

	if progress.Completed != "" {
		b.WriteString("## What Was Completed\n")
		b.WriteString(progress.Completed)
		b.WriteString("\n\n")
	}

	if progress.InProgress != "" {
		b.WriteString("## What Is In Progress\n")
		b.WriteString(progress.InProgress)
		b.WriteString("\n\n")
	}

	if progress.Remaining != "" {
		b.WriteString("## What Remains\n")
		b.WriteString(progress.Remaining)
		b.WriteString("\n\n")
	}

	if progress.Context != "" {
		b.WriteString("## Context\n")
		b.WriteString(progress.Context)
		b.WriteString("\n\n")
	}

	if gitLog != "" {
		b.WriteString("## Recent Git History\n")
		b.WriteString(gitLog)
		b.WriteString("\n\n")
	}

	b.WriteString("Continue from where the previous agent left off. Update .state/progress.md as you make progress.\n")

	return b.String(), nil
}

func recentGitLog(dir string) string {
	cmd := exec.Command("git", "-C", dir, "log", "--oneline", "-10")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
