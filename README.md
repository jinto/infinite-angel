# ina — Infinite Agent

[![Go](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

[한국어](README.ko.md)

**Daemon-supervised coding agents that never stop.**

_Launch an agent. Go to sleep. Wake up to working code._

[Quick Start](#quick-start) • [Skills](#skills) • [Pipeline](#pipeline) • [Acknowledgements](#acknowledgements)

---

## Quick Start

```bash
# Build
go build -o ina .
go build -o ina-mcp ./mcp/

# Configure Claude Code hooks + MCP
./ina setup

# Launch a daemon-supervised agent
./ina daemon &
./ina launch --path . --task "Add user authentication"

# Or use skills directly in Claude Code
/ina:autopilot Add user authentication
```

---

## What makes ina different

Other Claude Code plugins are **prompt libraries**. ina is **infrastructure**.

| | Prompt libraries | ina |
|---|---|---|
| Agent crashes | Gone forever | **Daemon restarts + resumes** |
| Context resets | Start over | **Pipeline state preserved** |
| Long-running tasks | Hope for the best | **Monitored + alerts** |
| Multiple agents | Manual | **Registry + coordination** |

---

## Skills

```
  THINK         PLAN          BUILD         REVIEW        SHIP
 ┌──────┐     ┌──────┐     ┌──────┐     ┌──────┐     ┌──────┐
 │ Idea │ ──▶ │ Plan │ ──▶ │ Code │ ──▶ │  QA  │ ──▶ │  PR  │
 │ Spec │     │ Tasks│     │ Test │     │ Fix  │     │ Merge│
 └──────┘     └──────┘     └──────┘     └──────┘     └──────┘
```

| Skill | Description |
|-------|-------------|
| `autopilot` | Full pipeline: think → plan → build → review → commit |
| `think` | Idea → spec (technical / business / improve) |
| `plan` | Consensus planning + TDD task breakdown |
| `build` | Execute tasks (direct / subagent / team) |
| `review` | Code review + fix-first auto-correction |
| `research` | Facet decomposition + parallel search |
| `design` | UI implementation + visual verification |
| `test` | Test runner + failure analysis + fix |
| `ship` | Create PR with auto-generated summary |
| `guard` | Safety guardrails for unattended execution |

---

## Pipeline

```
autopilot: think → plan → build → review → commit
                                    ↑         │
                                    └─────────┘ (loopback, max 3)
```

**Crash recovery** via `.state/pipeline.json` — the daemon restarts the agent and resumes from the recorded stage.

**Multi-perspective validation** — think and plan invoke Architect, Critic, and CEO subagents in parallel to validate specs and plans before execution.

**Execution delegation** — build auto-selects direct execution, subagent parallelism, or team coordination based on task independence.

---

## Architecture

```
┌─────────────────────────────────────────┐
│  Skill Layer (SKILL.md)                 │
│  In-session orchestration via           │
│  Claude Code native tools               │
│                                         │
│  autopilot / think / plan / build /     │
│  review / test / ship / guard           │
├─────────────────────────────────────────┤
│  Daemon Layer (Go binary)               │
│  Out-of-session process supervision     │
│  Crash recovery, restart, Discord alerts│
│                                         │
│  ina daemon / watcher / hooks / MCP     │
└─────────────────────────────────────────┘
```

---

## Build & Test

```bash
go build -o ina .
go build -o ina-mcp ./mcp/
go test ./... -count=1 -race

# E2E skill routing tests (costs API credits)
INFA_E2E=1 go test ./test/ -run TestSkillRouting -v
```

---

## Acknowledgements

Built with insights from:

- [oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode) — Multi-agent orchestration
- [superpowers](https://github.com/obra/superpowers) — Process discipline + TDD enforcement
- [gstack](https://github.com/garrytan/gstack) — Solo builder software factory
- [agent-skills](https://github.com/addyosmani/agent-skills) — Google SWE culture encoding

---

## License

[MIT](LICENSE)
