package hud

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// StatuslineStdin is the JSON Claude Code pipes to statusline commands.
type StatuslineStdin struct {
	CWD           string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
	Model         *Model `json:"model"`
	ContextWindow *ContextWindow `json:"context_window"`
}

type Model struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type ContextWindow struct {
	Size           int     `json:"context_window_size"`
	UsedPercentage float64 `json:"used_percentage"`
}

// Thresholds for context severity levels.
const (
	ThresholdWarning  = 70
	ThresholdCompress = 80
	ThresholdCritical = 85
)

// ANSI color codes.
const (
	green  = "\033[32m"
	yellow = "\033[33m"
	red    = "\033[31m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	reset  = "\033[0m"
)

type severity int

const (
	sevNormal severity = iota
	sevWarning
	sevCompress
	sevCritical
)

func classify(pct int) severity {
	switch {
	case pct >= ThresholdCritical:
		return sevCritical
	case pct >= ThresholdCompress:
		return sevCompress
	case pct >= ThresholdWarning:
		return sevWarning
	default:
		return sevNormal
	}
}

func (s severity) color() string {
	switch s {
	case sevCritical:
		return red
	case sevCompress, sevWarning:
		return yellow
	default:
		return green
	}
}

// DisabledFile is the flag file that disables the HUD.
var DisabledFile = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ina", "hud_disabled")
}()

// IsDisabled checks whether the HUD is turned off.
func IsDisabled() bool {
	_, err := os.Stat(DisabledFile)
	return err == nil
}

// Render reads Claude Code's statusline stdin and writes formatted output.
func Render(r io.Reader, w io.Writer) error {
	if IsDisabled() {
		return nil
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		fmt.Fprintln(w, "[ina] no stdin")
		return nil
	}

	var stdin StatuslineStdin
	if err := json.Unmarshal(data, &stdin); err != nil {
		fmt.Fprintln(w, "[ina] bad stdin")
		return nil
	}

	if stdin.ContextWindow == nil {
		fmt.Fprintln(w, "[ina]")
		return nil
	}

	pct := clamp(int(math.Round(stdin.ContextWindow.UsedPercentage)), 0, 100)
	sev := classify(pct)

	line := bold + "INA" + reset + " " + renderContextBar(pct, sev, 10)

	var extras []string
	if stdin.Model != nil && stdin.Model.DisplayName != "" {
		extras = append(extras, stdin.Model.DisplayName)
	}
	if stdin.CWD != "" {
		extras = append(extras, filepath.Base(stdin.CWD))
	}
	if len(extras) > 0 {
		line += dim + "  ·  " + strings.Join(extras, "  ·  ") + reset
	}

	fmt.Fprintln(w, line)

	if warning := renderWarning(pct); warning != "" {
		fmt.Fprintln(w, warning)
	}

	// Persist context percentage so hooks can read it.
	writeContextPct(pct)
	return nil
}

func renderContextBar(pct int, sev severity, barWidth int) string {
	filled := int(math.Round(float64(pct) / 100.0 * float64(barWidth)))
	empty := barWidth - filled
	c := sev.color()

	bar := c + strings.Repeat("█", filled) + dim + strings.Repeat("░", empty) + reset
	suffix := ""
	switch sev {
	case sevCritical:
		suffix = " CRITICAL"
	case sevCompress:
		suffix = " COMPRESS?"
	}

	return fmt.Sprintf("ctx:[%s]%s%d%%%s%s", bar, c, pct, suffix, reset)
}

func renderWarning(pct int) string {
	if pct < ThresholdCompress {
		return ""
	}
	icon := "!"
	c := yellow
	if pct >= 90 {
		icon = "!!"
		c = red
	}
	return fmt.Sprintf("%s%s[%s] ctx %d%% >= %d%% — run /compact%s",
		c, bold, icon, pct, ThresholdCompress, reset)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ContextPctFile is where the last known context percentage is stored.
var ContextPctFile = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".ina", "ctx_pct")
}()

func writeContextPct(pct int) {
	_ = os.WriteFile(ContextPctFile, []byte(fmt.Sprintf("%d", pct)), 0600)
}

// RenderFromStdin is a convenience for CLI use.
func RenderFromStdin() error {
	return Render(os.Stdin, os.Stdout)
}
