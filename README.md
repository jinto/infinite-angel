# ina — Infinite Agent

Daemon-supervised coding agents that never stop.

Go daemon (process watching, crash recovery, Discord alerts) + 10 Claude Code plugin skills with autopilot pipeline orchestration.

## Skills

| Skill | Description |
|-------|-------------|
| `/ina:autopilot` | Pipeline orchestrator — think → plan → build → review → commit |
| `/ina:think` | Idea refinement — technical / business / improve modes + multi-perspective validation |
| `/ina:plan` | Consensus-based planning + TDD task breakdown |
| `/ina:build` | Task executor with delegation (direct / subagent / team) |
| `/ina:review` | Multi-model code review + fix-first auto-correction |
| `/ina:research` | Facet decomposition + parallel search |
| `/ina:design` | Framework detection + implementation + visual verification |
| `/ina:test` | Standalone test runner + failure analysis + root cause fix |
| `/ina:ship` | PR creation with auto-generated summary |
| `/ina:guard` | Safety guardrails for unattended execution |

## Pipeline

```
autopilot: think → plan → build → review → commit
                                    ↑         │
                                    └─────────┘ (loopback, max 3)
```

Crash recovery via `.state/pipeline.json` — the daemon restarts the agent and resumes from the recorded stage.

## Build

```bash
go build -o ina .
go build -o ina-mcp ./mcp/
go test ./... -count=1 -race
```

## Acknowledgements

Built with insights from:

- [oh-my-claudecode](https://github.com/Yeachan-Heo/oh-my-claudecode) — Multi-agent orchestration (ralph, autopilot, team, ralplan)
- [superpowers](https://github.com/obra/superpowers) — Process discipline, Iron Laws, TDD enforcement
- [gstack](https://github.com/garrytan/gstack) — Solo builder software factory, browser daemon, design system
- [agent-skills](https://github.com/addyosmani/agent-skills) — Google SWE culture encoding, anti-rationalization, scope discipline

## License

MIT
