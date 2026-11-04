# agentcron

Schedule recurring agent work on your own machine. One Go binary: a daemon that fires jobs
on cron-style schedules, runs `claude`, `codex` or any command in a fresh git worktree,
records every run in a SQLite ledger, and gives each job a long-term memory note in an
[mdkb](https://github.com/ContractorKeith/mdkb) knowledge base. Manage it from the CLI, the
TUI, or let agents propose jobs over MCP (they stay paused until you approve them).

```toml
# ~/.config/agentcron/jobs/morning-issues.toml
schedule = "0 7 * * 1-5"
runner   = "claude"
repo     = "~/Projects/mdkb"
prompt   = "Triage issues opened since your last run. Note what you did in your memory note."
args     = ["--allowedTools", "Bash(gh issue *)"]
```

**Status:** planned, no code yet. See [the plan](docs/PLAN.md) and the
[glossary](CONTEXT.md). MIT licensed.

Run `agentcron tui` to browse jobs, runs and live logs in your terminal.
