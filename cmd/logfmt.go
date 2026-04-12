package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// stream-json event envelope
type streamEvent struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Message *streamMessage  `json:"message,omitempty"`
	Result  string          `json:"result,omitempty"`
	IsError bool            `json:"is_error,omitempty"`
	NumTurns int            `json:"num_turns,omitempty"`
	Usage   *streamUsage    `json:"usage,omitempty"`
	Cost    float64         `json:"total_cost_usd,omitempty"`
	// init fields
	CWD   string `json:"cwd,omitempty"`
	Model string `json:"model,omitempty"`
	// task_progress
	Description string `json:"description,omitempty"`
}

type streamMessage struct {
	Model   string         `json:"model,omitempty"`
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
	// tool_result fields
	ToolUseID string      `json:"tool_use_id,omitempty"`
	Content   interface{} `json:"content,omitempty"`
	IsError   bool        `json:"is_error,omitempty"`
}

type streamUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// prettyTail reads a log file, formats each JSON line for readability,
// and optionally follows new output.
func prettyTail(path string, numLines int, follow bool) error {
	lines, err := lastNLines(path, numLines)
	if err != nil {
		return err
	}

	for _, line := range lines {
		printFormatted(line)
	}

	if !follow {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.Seek(0, io.SeekEnd)

	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			reader.Reset(f)
			continue
		}
		printFormatted(strings.TrimRight(line, "\n"))
	}
}

func printFormatted(line string) {
	if line == "" {
		return
	}

	var ev streamEvent
	if err := json.Unmarshal([]byte(line), &ev); err != nil {
		fmt.Println(line)
		return
	}

	switch ev.Type {
	case "system":
		formatSystem(ev)
	case "assistant":
		formatAssistant(ev)
	case "user":
		formatToolResult(ev)
	case "result":
		formatResult(ev)
	}
	// skip: rate_limit_event, etc.
}

func formatSystem(ev streamEvent) {
	switch ev.Subtype {
	case "init":
		model := ev.Model
		if ev.Message != nil && ev.Message.Model != "" {
			model = ev.Message.Model
		}
		fmt.Printf("━━━ Session: %s @ %s ━━━\n", model, ev.CWD)
	case "task_progress":
		if ev.Description != "" {
			fmt.Printf("  ⏳ %s\n", ev.Description)
		}
	}
	// skip hook_started, hook_response
}

func formatAssistant(ev streamEvent) {
	if ev.Message == nil {
		return
	}
	for _, block := range ev.Message.Content {
		switch block.Type {
		case "text":
			if text := strings.TrimSpace(block.Text); text != "" {
				fmt.Println(text)
			}
		case "tool_use":
			summary := summarizeToolInput(block.Name, block.Input)
			fmt.Printf("▶ %s %s\n", block.Name, summary)
		}
	}
}

func formatToolResult(ev streamEvent) {
	if ev.Message == nil {
		return
	}
	for _, block := range ev.Message.Content {
		if block.Type != "tool_result" {
			continue
		}

		var content string
		switch v := block.Content.(type) {
		case string:
			content = v
		default:
			// could be structured — skip
			return
		}

		if block.IsError {
			first := firstLine(content)
			fmt.Printf("  ✗ %s\n", truncate(first, 120))
			return
		}

		lines := strings.Count(content, "\n")
		if lines > 3 {
			fmt.Printf("  ← %d lines\n", lines+1)
		} else if content != "" {
			trimmed := strings.TrimSpace(content)
			if trimmed != "" {
				fmt.Printf("  ← %s\n", truncate(trimmed, 120))
			}
		}
	}
}

func formatResult(ev streamEvent) {
	status := "done"
	if ev.IsError {
		status = "error"
	}
	fmt.Printf("━━━ %s (%d turns) ━━━\n", status, ev.NumTurns)
	if ev.Result != "" {
		fmt.Println(truncate(ev.Result, 200))
	}
}

func summarizeToolInput(name string, raw json.RawMessage) string {
	if raw == nil {
		return ""
	}

	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}

	switch name {
	case "Read":
		return getStr(m, "file_path")
	case "Write":
		return getStr(m, "file_path")
	case "Edit":
		return getStr(m, "file_path")
	case "Bash":
		cmd := getStr(m, "command")
		return truncate(cmd, 80)
	case "Glob":
		return getStr(m, "pattern")
	case "Grep":
		p := getStr(m, "pattern")
		path := getStr(m, "path")
		if path != "" {
			return p + " in " + path
		}
		return p
	case "Skill":
		return getStr(m, "skill")
	case "Agent":
		return getStr(m, "description")
	default:
		return ""
	}
}

func getStr(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func lastNLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var all []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB buffer for large JSON lines
	for scanner.Scan() {
		all = append(all, scanner.Text())
	}

	if len(all) <= n {
		return all, nil
	}
	return all[len(all)-n:], nil
}
