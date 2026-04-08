package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jinto/ina/agent"
	"github.com/jinto/ina/config"
	"github.com/jinto/ina/notify"
	"github.com/jinto/ina/state"
)

type Daemon struct {
	cfg        *config.Config
	registry   *agent.Registry
	notifier   *notify.Notifier
	listener   net.Listener
	hookServer *http.Server
	logger     *log.Logger
	stopCh     chan struct{}
	wg         sync.WaitGroup
}

func New(cfg *config.Config) (*Daemon, error) {
	if err := config.EnsureDir(); err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(config.LogFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}

	logger := log.New(logFile, "[ina] ", log.LstdFlags|log.Lshortfile)
	notifier := notify.New(cfg.Discord.WebhookURL, logger)

	return &Daemon{
		cfg:      cfg,
		registry: agent.NewRegistry(),
		notifier: notifier,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}, nil
}

func (d *Daemon) Run() error {
	if err := os.WriteFile(config.PidFile(), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		return fmt.Errorf("write pid: %w", err)
	}
	defer os.Remove(config.PidFile())

	os.Remove(config.SocketPath())

	ln, err := net.Listen("unix", config.SocketPath())
	if err != nil {
		return fmt.Errorf("listen socket: %w", err)
	}
	d.listener = ln
	os.Chmod(config.SocketPath(), 0600)
	defer ln.Close()
	defer os.Remove(config.SocketPath())

	d.restoreRegistry()
	d.cleanOldLogs()
	d.logger.Printf("daemon started, pid=%d, socket=%s", os.Getpid(), config.SocketPath())

	go d.acceptLoop()

	hookErrCh := make(chan error, 1)
	go func() {
		hookErrCh <- d.startHookServer()
	}()

	// Give the hook server a moment to bind. If it fails immediately
	// (port conflict, permission error), surface the error.
	select {
	case err := <-hookErrCh:
		// ErrServerClosed is normal during shutdown, not a startup failure.
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("hook server: %w", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Bound successfully; drain any later error in background.
		go func() {
			if err := <-hookErrCh; err != nil && err != http.ErrServerClosed {
				d.logger.Printf("hook server error: %v", err)
			}
		}()
	}

	<-d.stopCh
	d.logger.Println("daemon stopping")
	if d.hookServer != nil {
		d.hookServer.Close()
	}
	d.persistRegistry()
	d.wg.Wait()
	return nil
}

func (d *Daemon) Stop() {
	close(d.stopCh)
	if d.listener != nil {
		d.listener.Close()
	}
}

func (d *Daemon) persistRegistry() {
	if err := d.registry.SaveToFile(config.RegistryFile()); err != nil {
		d.logger.Printf("persist registry: %v", err)
	}
}

func (d *Daemon) removeAgent(id string) {
	d.registry.Remove(id)
	d.persistRegistry()
}

func (d *Daemon) acceptLoop() {
	for {
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.stopCh:
				return
			default:
				d.logger.Printf("accept error: %v", err)
				continue
			}
		}
		go d.handleConn(conn)
	}
}

func (d *Daemon) handleConn(conn net.Conn) {
	defer conn.Close()

	var cmd Command
	if err := json.NewDecoder(conn).Decode(&cmd); err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}

	d.logger.Printf("command: %s", cmd.Action)

	switch cmd.Action {
	case ActionStatus:
		d.handleStatus(conn)
	case ActionLaunch:
		d.handleLaunch(conn, cmd.Data)
	case ActionRestart:
		d.handleRestart(conn, cmd.Data)
	case ActionStop:
		d.handleStop(conn, cmd.Data)
	case ActionProgress:
		d.handleProgress(conn, cmd.Data)
	case ActionBlocked:
		d.handleBlocked(conn, cmd.Data)
	case ActionHook:
		d.handleHookForward(conn, cmd.Data)
	default:
		d.respond(conn, Response{Error: "unknown action: " + cmd.Action})
	}
}

func (d *Daemon) handleStatus(conn net.Conn) {
	snapshots := d.registry.All()
	data, err := json.Marshal(snapshots)
	if err != nil {
		d.respond(conn, Response{Error: fmt.Sprintf("marshal: %v", err)})
		return
	}
	d.respond(conn, Response{OK: true, Data: data})
}

func (d *Daemon) handleLaunch(conn net.Conn, raw json.RawMessage) {
	var req struct {
		Path     string `json:"path"`
		Task     string `json:"task"`
		Agent    string `json:"agent"`
		Name     string `json:"name"`
		Worktree bool   `json:"worktree"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}

	path, err := filepath.Abs(req.Path)
	if err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		d.respond(conn, Response{Error: "path does not exist or is not a directory: " + path})
		return
	}

	kind := agent.Kind(req.Agent)
	if kind == "" {
		kind = agent.Kind(d.cfg.Defaults.Agent)
	}
	if !agent.ValidKind(kind) {
		d.respond(conn, Response{Error: "invalid agent kind: " + string(kind)})
		return
	}

	name := req.Name
	if name == "" {
		name = filepath.Base(path)
	}
	if d.registry.NameExists(name) {
		base := name
		for i := 2; i <= 99; i++ {
			candidate := fmt.Sprintf("%s-%d", base, i)
			if !d.registry.NameExists(candidate) {
				name = candidate
				break
			}
		}
	}

	a := agent.New(name, kind, path, req.Task)

	if req.Worktree {
		wtPath, err := createWorktree(path, name)
		if err != nil {
			d.respond(conn, Response{Error: fmt.Sprintf("worktree: %v", err)})
			return
		}
		a.Worktree = wtPath
		a.CWD = wtPath
	}

	if err := state.Init(a.CWD, req.Task, string(kind)); err != nil {
		d.logger.Printf("warning: init state file: %v", err)
	}

	pid, err := d.launchProcess(a, false)
	if err != nil {
		d.respond(conn, Response{Error: fmt.Sprintf("launch failed: %v", err)})
		return
	}
	a.SetPID(pid)

	d.registry.Add(a)
	d.persistRegistry()

	d.wg.Add(1)
	go d.watchAgent(a)

	snap := a.Snapshot()
	d.notifier.AgentStarted(snap)
	d.logger.Printf("launched agent %s (pid=%d, kind=%s, cwd=%s)", snap.Name, snap.PID, snap.Kind, snap.CWD)
	d.respond(conn, Response{OK: true, Message: fmt.Sprintf("%s (pid=%d)", snap.Name, snap.PID)})
}

func (d *Daemon) handleRestart(conn net.Conn, raw json.RawMessage) {
	var req struct {
		Target string `json:"target"`
		Fresh  bool   `json:"fresh"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}

	a := d.findAgent(req.Target)
	if a == nil {
		d.respond(conn, Response{Error: "agent not found: " + req.Target})
		return
	}

	if err := d.restartAgent(a, req.Fresh); err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}

	snap := a.Snapshot()
	d.respond(conn, Response{OK: true, Message: fmt.Sprintf("%s restarted (pid=%d)", snap.Name, snap.PID)})
}

func (d *Daemon) handleStop(conn net.Conn, raw json.RawMessage) {
	var req struct {
		Target string `json:"target"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}

	a := d.findAgent(req.Target)
	if a == nil {
		d.respond(conn, Response{Error: "agent not found: " + req.Target})
		return
	}

	pid := a.PID()
	if pid > 0 {
		syscall.Kill(pid, syscall.SIGTERM)
	}
	a.SetState(agent.StateDead)
	if a.Worktree != "" {
		removeWorktree(filepath.Dir(filepath.Dir(a.Worktree)), a.Worktree)
	}
	d.removeAgent(a.ID)

	d.respond(conn, Response{OK: true, Message: a.Name + " stopped"})
}

func (d *Daemon) handleProgress(conn net.Conn, raw json.RawMessage) {
	var req struct {
		InProgress string `json:"in_progress"`
		Completed  string `json:"completed"`
		Remaining  string `json:"remaining"`
		Context    string `json:"context"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}

	if a := d.mostRecentlyActiveAgent(); a != nil {
		a.SetLastActive(time.Now())
		d.logger.Printf("progress from %s: %s", a.Name, req.InProgress)
	}

	d.respond(conn, Response{OK: true, Message: "progress recorded"})
}

func (d *Daemon) handleBlocked(conn net.Conn, raw json.RawMessage) {
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		d.respond(conn, Response{Error: err.Error()})
		return
	}

	if a := d.mostRecentlyActiveAgent(); a != nil {
		a.SetState(agent.StateBlocked)
		d.notifier.AgentBlocked(a.Snapshot())
		d.logger.Printf("agent %s blocked: %s", a.Name, req.Reason)
	}

	d.respond(conn, Response{OK: true, Message: "blocked status recorded"})
}

func (d *Daemon) mostRecentlyActiveAgent() *agent.Agent {
	var best *agent.Agent
	for _, a := range d.registry.Agents() {
		if a.GetState() != agent.StateRunning {
			continue
		}
		if best == nil || a.LastActive().After(best.LastActive()) {
			best = a
		}
	}
	return best
}

func (d *Daemon) findAgent(target string) *agent.Agent {
	return d.registry.FindByNameOrPrefix(target)
}

func (d *Daemon) respond(conn net.Conn, resp Response) {
	json.NewEncoder(conn).Encode(resp)
}

func (d *Daemon) restoreRegistry() {
	if err := d.registry.LoadFromFile(config.RegistryFile()); err != nil {
		d.logger.Printf("no registry to restore: %v", err)
		return
	}

	var stale []string
	for _, a := range d.registry.Agents() {
		if agent.IsAlive(a.PID()) {
			a.SetState(agent.StateRunning)
			d.wg.Add(1)
			go d.watchAgent(a)
			d.logger.Printf("restored agent %s (pid=%d)", a.Name, a.PID())
		} else {
			d.logger.Printf("stale agent %s (pid=%d) removed", a.Name, a.PID())
			stale = append(stale, a.ID)
		}
	}

	for _, id := range stale {
		d.registry.Remove(id)
	}
	d.persistRegistry()
}

func (d *Daemon) cleanOldLogs() {
	logBaseDir := filepath.Join(config.DataDir(), "logs")
	entries, err := os.ReadDir(logBaseDir)
	if err != nil {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -d.cfg.Daemon.MaxLogAge())
	var removed int

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentDir := filepath.Join(logBaseDir, entry.Name())
		files, err := os.ReadDir(agentDir)
		if err != nil {
			continue
		}
		for _, f := range files {
			info, err := f.Info()
			if err != nil {
				continue
			}
			if info.ModTime().Before(cutoff) {
				os.Remove(filepath.Join(agentDir, f.Name()))
				removed++
			}
		}
	}

	if removed > 0 {
		d.logger.Printf("cleaned %d old log files (older than %d days)", removed, d.cfg.Daemon.MaxLogAge())
	}
}

func createWorktree(repoDir, name string) (string, error) {
	wtPath := filepath.Join(repoDir, ".worktrees", name)
	if err := os.MkdirAll(filepath.Dir(wtPath), 0700); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "worktree", "add", wtPath, "HEAD")
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git worktree add: %s: %w", out, err)
	}
	return wtPath, nil
}

func removeWorktree(repoDir, wtPath string) {
	exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", wtPath).Run()
}

func StopRunning() error {
	data, err := os.ReadFile(config.PidFile())
	if err != nil {
		return fmt.Errorf("daemon not running (no pid file)")
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return fmt.Errorf("invalid pid file")
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return proc.Signal(syscall.SIGTERM)
}
