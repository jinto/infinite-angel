# OMZ Tasks

## Current Sprint

- [x] `omz log` command — tail agent or daemon logs (`--follow` support)
- [x] `--fresh` restart flag — skip state injection, restart with original task only
- [x] Remove dead `session/tmux.go` — zero imports across codebase

## Backlog (completed)

- [x] `omz install` command — generate launchd plist for auto-start
- [x] Unit tests — state parser, agent registry, config (3 test files, race-safe)
- [x] Git worktree isolation — `--worktree` creates isolated worktree, cleanup on death/stop
- [x] Log rotation — clean old logs on startup (configurable max_log_age_days)
- [x] Codex CLI verification — exec.LookPath check before launch
- [x] Agent name uniqueness — auto-suffix on collision (api → api-2)

## Phase 2: Event-Driven Architecture

- [x] HTTP hook listener — daemon receives Claude Code events (SessionStart/End, Stop, PostToolUse)
- [x] `omz setup` command — auto-configure hooks + MCP server in Claude Code settings
- [x] `--continue` restart — resume sessions with conversation context preserved
- [x] MCP server (omz-mcp) — report_progress, mark_blocked, check_agents tools
- [x] `omz attach` command — tail live agent log output
