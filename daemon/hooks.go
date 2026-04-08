package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/jinto/ina/agent"
)

type hookPayload struct {
	SessionID     string `json:"session_id"`
	CWD           string `json:"cwd"`
	HookEventName string `json:"hook_event_name"`
	ToolName      string `json:"tool_name,omitempty"`
}

type hookProgressPayload struct {
	CWD       string   `json:"cwd"`
	Completed []string `json:"completed"`
	Current   string   `json:"in_progress"`
	Remaining []string `json:"remaining"`
	Context   string   `json:"context"`
}

type hookBlockedPayload struct {
	CWD    string `json:"cwd"`
	Reason string `json:"reason"`
}

func decodeHook[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	var payload T
	if r.Body == nil {
		w.WriteHeader(http.StatusOK)
		return payload, false
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		// Hooks are best-effort; return 200 so Claude Code doesn't log errors.
		w.WriteHeader(http.StatusOK)
		return payload, false
	}
	return payload, true
}

func (d *Daemon) startHookServer() error {
	mux := http.NewServeMux()
	mux.HandleFunc("/hooks/session-start", d.handleHookSessionStart)
	mux.HandleFunc("/hooks/session-end", d.handleHookSessionEnd)
	mux.HandleFunc("/hooks/stop", d.handleHookStop)
	mux.HandleFunc("/hooks/post-tool-use", d.handleHookToolUse)
	mux.HandleFunc("/hooks/progress", d.handleHookProgress)
	mux.HandleFunc("/hooks/blocked", d.handleHookBlocked)

	addr := fmt.Sprintf("127.0.0.1:%d", d.cfg.Daemon.GetHookPort())
	d.hookServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
	}

	d.logger.Printf("hook server listening on %s", addr)
	return d.hookServer.ListenAndServe()
}

// Core hook event processors — shared by HTTP and socket handlers.

func (d *Daemon) processSessionStart(p hookPayload) {
	d.logger.Printf("hook: session-start session=%s cwd=%s", p.SessionID, p.CWD)
	if p.CWD == "" {
		return
	}
	if a := d.registry.FindByCWD(p.CWD); a == nil {
		a = agent.New(nameFromCWD(p.CWD), agent.KindClaude, p.CWD, "auto-detected session")
		a.SetPID(0)
		d.registry.Add(a)
		d.persistRegistry()
		d.logger.Printf("auto-registered agent %s from hook", a.Name)
	}
}

func (d *Daemon) processSessionEnd(p hookPayload) {
	if a := d.registry.FindByCWD(p.CWD); a != nil {
		a.SetState(agent.StateDead)
	}
}

func (d *Daemon) processStop(p hookPayload) {
	if a := d.registry.FindByCWD(p.CWD); a != nil {
		a.SetLastActive(time.Now())
	}
}

func (d *Daemon) processToolUse(p hookPayload) {
	if a := d.registry.FindByCWD(p.CWD); a != nil {
		a.TouchLastActive(time.Now())
	}
}

// HTTP hook handlers — thin wrappers around core processors.

func (d *Daemon) handleHookSessionStart(w http.ResponseWriter, r *http.Request) {
	p, ok := decodeHook[hookPayload](w, r)
	if !ok {
		return
	}
	d.processSessionStart(p)
	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) handleHookSessionEnd(w http.ResponseWriter, r *http.Request) {
	p, ok := decodeHook[hookPayload](w, r)
	if !ok {
		return
	}
	d.processSessionEnd(p)
	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) handleHookStop(w http.ResponseWriter, r *http.Request) {
	p, ok := decodeHook[hookPayload](w, r)
	if !ok {
		return
	}
	d.processStop(p)
	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) handleHookToolUse(w http.ResponseWriter, r *http.Request) {
	p, ok := decodeHook[hookPayload](w, r)
	if !ok {
		return
	}
	d.processToolUse(p)
	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) handleHookProgress(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeHook[hookProgressPayload](w, r)
	if !ok {
		return
	}

	if a := d.registry.FindByCWD(req.CWD); a != nil {
		a.SetLastActive(time.Now())
		d.logger.Printf("hook: progress from %s — %s", a.Name, req.Current)
	}

	w.WriteHeader(http.StatusOK)
}

func (d *Daemon) handleHookBlocked(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeHook[hookBlockedPayload](w, r)
	if !ok {
		return
	}

	if a := d.registry.FindByCWD(req.CWD); a != nil {
		a.SetState(agent.StateBlocked)
		d.notifier.AgentBlocked(a.Snapshot())
		d.logger.Printf("hook: agent %s blocked — %s", a.Name, req.Reason)
	}

	w.WriteHeader(http.StatusOK)
}

// handleHookForward processes hook events forwarded from `ina hook <event>`
// via the unix socket, instead of the HTTP hook server.
func (d *Daemon) handleHookForward(conn net.Conn, raw json.RawMessage) {
	var req struct {
		Event string          `json:"event"`
		Body  json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		d.respond(conn, Response{OK: true})
		return
	}

	var payload hookPayload
	if len(req.Body) > 0 {
		json.Unmarshal(req.Body, &payload)
	}

	switch req.Event {
	case "session-start":
		d.processSessionStart(payload)
	case "session-end":
		d.processSessionEnd(payload)
	case "stop":
		d.processStop(payload)
	case "post-tool-use":
		d.processToolUse(payload)
	default:
		d.logger.Printf("hook: unknown event %q via socket", req.Event)
	}

	d.respond(conn, Response{OK: true})
}

func nameFromCWD(cwd string) string {
	name := filepath.Base(cwd)
	if name == "." || name == "/" {
		return "unknown"
	}
	return name
}
