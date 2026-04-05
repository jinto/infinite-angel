package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jinto/ina/agent"
)

const (
	ColorRed    = 0xFF0000 // died
	ColorYellow = 0xFFAA00 // stalled/blocked
	ColorGreen  = 0x00CC00 // milestone
	ColorBlue   = 0x0088FF // started/restarted
)

type Notifier struct {
	webhookURL string
	logger     *log.Logger
	client     *http.Client
}

func New(webhookURL string, logger *log.Logger) *Notifier {
	return &Notifier{
		webhookURL: webhookURL,
		logger:     logger,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (n *Notifier) AgentStarted(snap agent.Snapshot) {
	n.sendEmbed(embed{
		Title:       fmt.Sprintf("Agent Started: %s", snap.Name),
		Description: snap.TaskDesc,
		Color:       ColorBlue,
		Fields: []field{
			{Name: "Agent", Value: string(snap.Kind), Inline: true},
			{Name: "PID", Value: fmt.Sprintf("%d", snap.PID), Inline: true},
			{Name: "CWD", Value: snap.CWD, Inline: false},
		},
	})
}

func (n *Notifier) AgentDied(snap agent.Snapshot) {
	n.sendEmbed(embed{
		Title:       fmt.Sprintf("Agent Died: %s", snap.Name),
		Description: fmt.Sprintf("Process %d is no longer running.", snap.PID),
		Color:       ColorRed,
		Fields: []field{
			{Name: "Task", Value: snap.TaskDesc, Inline: false},
			{Name: "Uptime", Value: time.Since(snap.StartedAt).Round(time.Second).String(), Inline: true},
			{Name: "Restarts", Value: fmt.Sprintf("%d", snap.RestartCount), Inline: true},
		},
	})
}

func (n *Notifier) AgentStalled(snap agent.Snapshot) {
	n.sendEmbed(embed{
		Title:       fmt.Sprintf("Agent Stalled: %s", snap.Name),
		Description: fmt.Sprintf("No activity for %s.", time.Since(snap.LastActive).Round(time.Second)),
		Color:       ColorYellow,
		Fields: []field{
			{Name: "Task", Value: snap.TaskDesc, Inline: false},
			{Name: "PID", Value: fmt.Sprintf("%d", snap.PID), Inline: true},
		},
	})
}

func (n *Notifier) AgentBlocked(snap agent.Snapshot) {
	n.sendEmbed(embed{
		Title:       fmt.Sprintf("Agent Blocked: %s", snap.Name),
		Description: "Agent reports it needs human input.",
		Color:       ColorYellow,
		Fields: []field{
			{Name: "Task", Value: snap.TaskDesc, Inline: false},
			{Name: "CWD", Value: snap.CWD, Inline: false},
		},
	})
}

func (n *Notifier) AgentRestarted(snap agent.Snapshot) {
	n.sendEmbed(embed{
		Title:       fmt.Sprintf("Agent Restarted: %s", snap.Name),
		Description: fmt.Sprintf("Attempt %d, new PID %d.", snap.RestartCount, snap.PID),
		Color:       ColorBlue,
		Fields: []field{
			{Name: "Task", Value: snap.TaskDesc, Inline: false},
		},
	})
}

func (n *Notifier) Send(message string) {
	n.sendRaw(map[string]string{"content": message})
}

// Discord embed types

type embed struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Color       int     `json:"color"`
	Fields      []field `json:"fields,omitempty"`
}

type field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func (n *Notifier) sendEmbed(e embed) {
	n.sendRaw(map[string]interface{}{
		"embeds": []embed{e},
	})
}

func (n *Notifier) sendRaw(payload interface{}) {
	if n.webhookURL == "" {
		return
	}

	body, err := json.Marshal(payload)
	if err != nil {
		n.logger.Printf("notify marshal error: %v", err)
		return
	}

	resp, err := n.client.Post(n.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		n.logger.Printf("notify send error: %v", err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		n.logger.Printf("notify discord returned %d", resp.StatusCode)
	}
}
