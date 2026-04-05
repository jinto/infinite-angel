package daemon

import (
	"fmt"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jinto/ina/agent"
	"github.com/jinto/ina/state"
)

func (d *Daemon) watchAgent(a *agent.Agent) {
	defer d.wg.Done()

	interval := d.cfg.Daemon.CheckIntervalDuration()
	threshold := d.cfg.Daemon.IdleThresholdDuration()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.checkAgent(a, threshold)

			if a.GetState() == agent.StateDead {
				d.handleDeath(a)
				return
			}
		}
	}
}

func (d *Daemon) checkAgent(a *agent.Agent, threshold time.Duration) {
	pid := a.PID()

	if !agent.IsAlive(pid) {
		a.SetState(agent.StateDead)
		d.logger.Printf("agent %s (pid=%d) died", a.Name, pid)
		return
	}

	progress, err := state.Read(a.CWD)
	if err == nil && progress.Blocked {
		if a.GetState() != agent.StateBlocked {
			a.SetState(agent.StateBlocked)
			d.notifier.AgentBlocked(a.Snapshot())
			d.logger.Printf("agent %s is blocked", a.Name)
		}
		return
	}

	lastActive := agent.LatestActivity(a.CWD)
	if !lastActive.IsZero() {
		a.SetLastActive(lastActive)
	}

	if time.Since(a.LastActive()) > threshold {
		if a.GetState() != agent.StateStalled {
			a.SetState(agent.StateStalled)
			d.notifier.AgentStalled(a.Snapshot())
			d.logger.Printf("agent %s stalled (no activity for %v)", a.Name, threshold)
		}
	} else {
		a.SetState(agent.StateRunning)
	}
}

func (d *Daemon) handleDeath(a *agent.Agent) {
	snap := a.Snapshot()
	d.notifier.AgentDied(snap)

	if !d.cfg.Daemon.AutoRestart {
		d.logger.Printf("auto-restart disabled, agent %s stays dead", a.Name)
		d.cleanupWorktree(a)
		d.removeAgent(a.ID)
		return
	}

	if snap.RestartCount >= d.cfg.Daemon.MaxRestarts {
		d.notifier.Send(fmt.Sprintf("Agent **%s** exceeded max restarts (%d). Manual intervention needed.", a.Name, d.cfg.Daemon.MaxRestarts))
		d.logger.Printf("agent %s exceeded max restarts", a.Name)
		d.cleanupWorktree(a)
		d.removeAgent(a.ID)
		return
	}

	d.logger.Printf("auto-restarting agent %s (attempt %d/%d)", a.Name, snap.RestartCount+1, d.cfg.Daemon.MaxRestarts)

	if err := d.restartAgent(a, false); err != nil {
		d.logger.Printf("restart failed: %v", err)
		d.notifier.Send(fmt.Sprintf("Failed to restart agent **%s**: %v", a.Name, err))
		d.cleanupWorktree(a)
		d.removeAgent(a.ID)
		return
	}

	d.wg.Add(1)
	go d.watchAgent(a)
}

func (d *Daemon) cleanupWorktree(a *agent.Agent) {
	if a.Worktree == "" {
		return
	}
	removeWorktree(filepath.Dir(filepath.Dir(a.Worktree)), a.Worktree)
	d.logger.Printf("cleaned up worktree %s", a.Worktree)
}

func (d *Daemon) restartAgent(a *agent.Agent, fresh bool) error {
	oldPID := a.PID()
	if agent.IsAlive(oldPID) {
		syscall.Kill(oldPID, syscall.SIGTERM)
		time.Sleep(2 * time.Second)
		if agent.IsAlive(oldPID) {
			syscall.Kill(oldPID, syscall.SIGKILL)
		}
	}

	a.IncrRestarts()

	if fresh {
		if err := state.Init(a.CWD, a.TaskDesc, string(a.Kind)); err != nil {
			d.logger.Printf("warning: reset state file: %v", err)
		}
	}

	pid, err := d.launchProcess(a, fresh)
	if err != nil {
		return fmt.Errorf("launch: %w", err)
	}

	a.SetPID(pid)
	a.SetState(agent.StateRunning)
	a.SetLastActive(time.Now())

	snap := a.Snapshot()
	d.notifier.AgentRestarted(snap)
	d.logger.Printf("agent %s restarted (pid=%d, attempt=%d)", a.Name, snap.PID, snap.RestartCount)

	return nil
}
