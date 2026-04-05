package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// IsAlive checks if a process with the given PID exists.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// LastFileActivity returns the most recent modification time in dir (non-recursive, skipping .git).
func LastFileActivity(dir string) time.Time {
	var latest time.Time

	entries, err := os.ReadDir(dir)
	if err != nil {
		return latest
	}

	for _, e := range entries {
		if e.Name() == ".git" || e.Name() == ".state" || e.Name() == "node_modules" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
	}

	return latest
}

// LastGitCommit returns the timestamp of the most recent git commit in dir.
func LastGitCommit(dir string) time.Time {
	cmd := exec.Command("git", "-C", dir, "log", "-1", "--format=%ct")
	out, err := cmd.Output()
	if err != nil {
		return time.Time{}
	}

	ts, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return time.Time{}
	}

	return time.Unix(ts, 0)
}

// LastStateUpdate returns the mtime of the state file.
func LastStateUpdate(dir string) time.Time {
	info, err := os.Stat(filepath.Join(dir, ".state", "progress.md"))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// LatestActivity returns the most recent of all activity signals.
func LatestActivity(dir string) time.Time {
	candidates := []time.Time{
		LastFileActivity(dir),
		LastGitCommit(dir),
		LastStateUpdate(dir),
	}

	var latest time.Time
	for _, t := range candidates {
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}
