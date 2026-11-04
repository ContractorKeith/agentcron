# agentcron glossary

- **Job** — a named, recurring unit of agent work defined by one TOML file in `jobs/`. *Avoid:* task, cron, automation.
- **Schedule** — the job's timing expression: 5-field cron, `every <dur>`, `at <time>` or a macro, evaluated in the job's timezone. *Avoid:* trigger, interval.
- **Fire** — one instant a schedule says the job is due. *Avoid:* tick, occurrence.
- **Run** — one execution of a job, with a row in the ledger, logs and a worktree. *Avoid:* execution, attempt, invocation.
- **Trigger** — why a run exists: `schedule`, `manual`, `catchup` or `mcp`.
- **Runner** — the program a run launches: `claude`, `codex` or `command`. *Avoid:* agent, backend, adapter (the adapter is the code that builds the runner's argv).
- **Ledger** — the SQLite `runs` table; the record of every run and its state. *Avoid:* history, log.
- **Run state** — one of queued, claimed, running, succeeded, failed, timed_out, cancelled, attention, skipped, blocked_config, unknown.
- **Attention** — a run that exited 0 but did not meet the job's `expect`. *Avoid:* warning, soft failure.
- **Blocked config** — a run stopped at preflight; nothing was launched.
- **Unknown** — a run whose process vanished while the daemon was down; never re-run automatically.
- **Run worktree** — the git worktree a run works in, on branch `agentcron/<job>/<run-id>`; kept only when dirty or ahead. *Avoid:* sandbox, checkout.
- **Preamble** — the short header agentcron prepends to the prompt: job, run id, fire time, last run state and output path, memory note.
- **Memory note** — the job's note `agentcron/<job>` in the mdkb knowledge base, reachable through mdkb's MCP during a run. *Avoid:* memory store, vector DB.
- **Daemon** — the long-running `agentcron daemon` process that fires jobs. *Avoid:* server, service (the service is the systemd/launchd unit that starts the daemon).
- **Tick** — one scheduling pass: reconcile, enqueue due fires, claim and start runs. `agentcron tick` runs exactly one.
- **Catch-up** — what happens to fires missed while no daemon ran: `one` enqueues a single run, `none` skips them.
- **Approval** — a human `approve` that lets a job created through MCP fire. *Avoid:* review.
