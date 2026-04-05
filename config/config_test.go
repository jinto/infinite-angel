package config

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := Default()

	if cfg.Daemon.CheckInterval != "30s" {
		t.Errorf("CheckInterval = %q, want %q", cfg.Daemon.CheckInterval, "30s")
	}
	if cfg.Daemon.MaxRestarts != 3 {
		t.Errorf("MaxRestarts = %d, want 3", cfg.Daemon.MaxRestarts)
	}
	if !cfg.Daemon.AutoRestart {
		t.Error("AutoRestart should be true by default")
	}
	if cfg.Defaults.Agent != "claude" {
		t.Errorf("Agent = %q, want %q", cfg.Defaults.Agent, "claude")
	}
}

func TestCheckIntervalDuration(t *testing.T) {
	dc := DaemonConfig{CheckInterval: "10s"}
	if d := dc.CheckIntervalDuration(); d != 10*time.Second {
		t.Errorf("CheckIntervalDuration = %v, want 10s", d)
	}

	dc.CheckInterval = "invalid"
	if d := dc.CheckIntervalDuration(); d != 30*time.Second {
		t.Errorf("CheckIntervalDuration fallback = %v, want 30s", d)
	}
}

func TestIdleThresholdDuration(t *testing.T) {
	dc := DaemonConfig{IdleThreshold: "10m"}
	if d := dc.IdleThresholdDuration(); d != 10*time.Minute {
		t.Errorf("IdleThresholdDuration = %v, want 10m", d)
	}

	dc.IdleThreshold = ""
	if d := dc.IdleThresholdDuration(); d != 5*time.Minute {
		t.Errorf("IdleThresholdDuration fallback = %v, want 5m", d)
	}
}

func TestMaxLogAge(t *testing.T) {
	dc := DaemonConfig{MaxLogAgeDays: 0}
	if dc.MaxLogAge() != 7 {
		t.Errorf("MaxLogAge default = %d, want 7", dc.MaxLogAge())
	}

	dc.MaxLogAgeDays = 30
	if dc.MaxLogAge() != 30 {
		t.Errorf("MaxLogAge = %d, want 30", dc.MaxLogAge())
	}

	dc.MaxLogAgeDays = -1
	if dc.MaxLogAge() != 7 {
		t.Errorf("MaxLogAge negative = %d, want 7", dc.MaxLogAge())
	}
}

func TestLoadMissingFile(t *testing.T) {
	// Load should return defaults when file doesn't exist
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Daemon.MaxRestarts != 3 {
		t.Errorf("MaxRestarts = %d, want 3", cfg.Daemon.MaxRestarts)
	}
}
