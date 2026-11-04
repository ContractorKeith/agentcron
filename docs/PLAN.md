# agentcron — Build Plan

Status: planned, no code yet (2026-11-04). Sources studied: Hermes Agent cron, Agenvoy
scheduler, OpenClaw cron, omarchy-agentcron, reeschan/agentcron; process model mdkb.

## 1. Goal

agentcron is a local scheduler for recurring agent work. You (or an agent, with your
approval) define a **job**: a schedule, a runner (`claude`, `codex` or any command), a
prompt and optionally a repo. The daemon fires each job on time, runs it in a fresh git
worktree with the permissions the job names, records every run in a SQLite ledger with
its output, and hands the agent a per-job **memory note** in a shared mdkb knowledge base
so the work builds on itself. It is for one person running Claude Code and Codex on their
own Mac or Linux box ("check GitHub issues every morning", "summarize Basecamp", "reindex
the KB") and closes the gap between one-off sessions and always-on automation.

v1 is done when every row marked **v1** in §3 works on macOS and Linux (amd64, arm64) and
`scripts/accept.sh` passes from a clean clone.

## 2. Inspirations and what to skip

| Source | Copy | Skip |
|---|---|---|
| Hermes Agent cron (MIT) | Schedule grammar (`every 2h`, 5-field cron, ISO time, per-job timezone); run ledger where an unknown outcome is never re-run; agent scheduling off by default and refused inside a scheduled run; `tick` for external cron | Its own agent loop and chat delivery platforms |
| omarchy-agentcron (MIT) | "Exit 0 is not success": `expect` turns a no-op run into `attention` | systemd timers as the scheduler (no macOS, no ledger) |
| Agenvoy (AGPL) | Agents can create schedules through a tool | Allow-all scheduled runs; a bespoke vector store; AGPL code (read the README only) |
| reeschan/agentcron | Passing `--permission-mode`/`--allowedTools` per job | JSON config, no worktrees, no memory |
| mdkb (ours) | Memory backend, process shape, Go stack, `accept.sh`, MCP SDK | — |

## 3. Feature matrix

**v1** = in scope now. **Later** = after v1. **Drop** = deliberately never.

| # | Feature | Decision | Milestone | Notes |
|---|---|---|---|---|
| 1 | Go module, cobra CLI skeleton, `version`, `init`, `doctor` stub, XDG paths, config.toml | v1 | M0 | |
| 2 | SQLite schema + migrations (`modernc.org/sqlite`), store seam, clock seam | v1 | M0 | Fake clock in every scheduler test |
| 3 | Fake `claude`/`codex`/`mdkb` test binaries, CI matrix (ubuntu, macos), `accept.sh` | v1 | M0 | Tests never call real models |
| 4 | Job files: one TOML per job in `jobs/`, validation, `add`/`ls`/`show`/`rm`/`pause`/`resume` | v1 | M1 | ADR 0001 |
| 5 | Schedules: 5-field cron, `every <dur>`, `at <RFC3339>`, `@daily`-style macros, per-job `timezone`, `next` | v1 | M1 | DST: skip nonexistent, fire once on repeated |
| 6 | Runners: `claude`, `codex`, `command` adapters building argv from job fields | v1 | M2 | Exact flags in §6 |
| 7 | Run worktree per run from `repo` + `base`, branch `agentcron/<job>/<run>`; scratch dir when no repo | v1 | M2 | Clean worktrees removed, dirty ones kept |
| 8 | Preflight → `blocked_config` without launching (missing runner, repo, bad base) | v1 | M2 | |
| 9 | Ledger states, timeouts with process-group kill, `cancel`, stdout/stderr logs, `expect` → `attention` | v1 | M2 | |
| 10 | Prompt preamble (job, run id, fire time, last run state and output path, memory note id) | v1 | M2 | Memory lines added in M4 |
| 11 | `run <job> [--now] [--wait]`, `runs`, `logs [--follow]`, `worktrees`, `prune` | v1 | M2 | `run` without `--now` enqueues |
| 12 | Daemon: single-instance lock, tick loop, queue claims, global concurrency, no per-job overlap | v1 | M3 | |
| 13 | Catch-up policy (`one` default, `none`), restart reconcile (`running` with dead pid → `unknown`) | v1 | M3 | Unknown is never re-run |
| 14 | `tick` (one pass for external cron), retention of run logs (`keep_runs`, default 50) | v1 | M3 | |
| 15 | `service install|uninstall|status` for systemd user units and launchd agents, PATH captured at install | v1 | M3 | |
| 16 | Memory via mdkb: detect binary, per-job note `agentcron/<job>`, inject mdkb MCP into claude/codex runs, `memory <job>` | v1 | M4 | ADR 0002 |
| 17 | stdio MCP server: read tools; write tools behind `--allow-write`; recursion guard; agent jobs start paused | v1 | M5 | ADR 0003 |
| 18 | TUI (Bubble Tea v2): jobs, runs, log view, run-now, pause/resume, approve agent jobs | v1 | M6 | Reads the store; writes through `internal/app` |
| 19 | Release workflow (four tarballs + checksums), Homebrew formula, README quick start | v1 | M7 | Same shape as mdkb |
| 20 | Container or bubblewrap/sandbox-exec isolation per run | Later | | Worktree + runner permissions first |
| 21 | Notifications on attention/failure (desktop notify, webhook) | Later | | |
| 22 | Job chaining (run B after A succeeds), retries with backoff for `failed` | Later | | |
| 23 | Remote/headless multi-machine scheduling | Later | | |
| 24 | Chat delivery (Telegram, Discord, Slack) | Drop | | Hermes/OpenClaw territory |
| 25 | Built-in vector store or LLM calls of its own | Drop | | mdkb and the runner do that |
| 26 | Task graphs, spec-to-PR orchestration | Drop | | That is agentpm |
| 27 | Windows | Drop | | |

## 4. Architecture

```
cmd/agentcron/        cobra commands; flags and output only, no SQL or file access
internal/app/         use cases shared by CLI, MCP and TUI (add job, enqueue, cancel, ...)
internal/config/      XDG paths, config.toml, precedence flag > env > file > default
internal/jobs/        job TOML files: parse, validate, atomic write; only package touching jobs/
internal/schedule/    expression parsing and next-fire maths; pure, takes a time and a zone
internal/store/       SQLite ledger and job state; only package importing database/sql
internal/runner/      preflight, worktree lifecycle (git via exec), adapters, process control, logs
internal/memory/      mdkb detection, note bootstrap, MCP config for runs
internal/daemon/      lock, tick loop, claims, concurrency, catch-up, reconcile, retention
internal/service/     systemd/launchd unit rendering and install
internal/mcpserver/   stdio MCP tools over internal/app
internal/tui/         Bubble Tea model over internal/app
internal/clock/       Clock interface; real and fake
testdata/fakes/       fake claude, codex, mdkb, git-hook programs built by tests
testdata/jobs/        fixture job files; testdata/golden/ JSON goldens
```

Rules:
- Job files are the source of truth for job definitions; the database holds job state
  (paused, next fire, last run) and the run ledger. Deleting the database loses history,
  never jobs.
- Only `internal/store` imports `database/sql`; only `internal/jobs` reads or writes job
  files. `cmd/`, `internal/mcpserver` and `internal/tui` call `internal/app` only.
- No package calls `time.Now` directly; everything takes an `internal/clock.Clock`.
- Every state change of a run is one `BEGIN IMMEDIATE` transaction; a claim is
  `UPDATE runs SET state='claimed' WHERE id=? AND state='queued'` and checks rows affected.
- Child processes run in their own process group; timeout and cancel signal the group
  (SIGTERM, 10 s grace, SIGKILL).
- v1 contains runs by process group; a descendant that calls setsid escapes teardown (OS sandboxing is Later).

### Dependencies and why

| Module | Why |
|---|---|
| `github.com/spf13/cobra` | Same CLI shape as mdkb and machine-monitor |
| `modernc.org/sqlite` | Pure Go, keeps cross-compiles one command |
| `github.com/BurntSushi/toml` | Job and config files humans edit |
| `github.com/robfig/cron/v3` (parser only) | Battle-tested 5-field cron parsing with time zones; we wrap it, never use its runner |
| `github.com/modelcontextprotocol/go-sdk` | Same MCP SDK as mdkb |
| `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2` | Same TUI stack as machine-monitor |

## 5. Data model

Paths (Linux `$XDG_*`, macOS the same variables with these fallbacks):
- config `~/.config/agentcron/config.toml`, jobs `~/.config/agentcron/jobs/<name>.toml`
- data `~/.local/share/agentcron/`: `agentcron.db`, `runs/<run-id>/{stdout,stderr}.log`,
  `runs/<run-id>/mcp.json`, `worktrees/<job>/<run-id>/`, `daemon.lock`

Memory configuration in `config.toml`:

```toml
mdkb_path = ""  # empty: find mdkb on PATH; otherwise the executable path
mdkb_root = ""  # empty: MDKB_ROOT, then mdkb's own config/default
```

The config layer expands `~` and resolves configured paths to absolute paths.
An unset KB root stays unset so mdkb retains its own root precedence. New jobs
created by `add` default memory to mdkb presence; `--memory` and `--no-memory`
explicitly override that default.

**Job** — named by its file name: `^[a-z0-9][a-z0-9-]{0,62}$`.

```toml
schedule = "0 7 * * 1-5"         # or "every 2h", "at 2026-11-05T09:00:00-05:00", "@daily"
timezone = "America/New_York"    # default: local zone
runner   = "claude"              # claude | codex | command
prompt   = "Triage new issues in this repo. Record what you did in your memory note."
# prompt_file = "prompts/triage.md"   # relative to the jobs dir; exactly one of prompt/prompt_file
repo     = "~/Projects/mdkb"     # optional; absent = scratch dir per run
base     = "origin/main"         # default: the repo's origin HEAD, else HEAD
args     = ["--permission-mode", "acceptEdits", "--allowedTools", "Bash(gh issue *)"]
command  = []                    # runner = "command" only: argv, prompt on stdin
timeout  = "30m"
expect   = "(?m)^DONE"           # optional regex on stdout; no match = attention
memory   = true                  # default true when mdkb is available
catchup  = "one"                 # one | none
created_by = "cli"               # cli | mcp (set by agentcron)
```

**Job state** (SQLite `job_state`): name, paused, approved (false for `created_by = "mcp"`
until a human approves), next_fire, last_run_id, updated_at.

**Run** (SQLite `runs`): id `r_<UTC yyyymmddThhmmssZ>_<4 base32>`, job, trigger
(`schedule|manual|catchup|mcp`), scheduled_for, state, pid, exit_code, started_at,
ended_at, worktree, kept (bool), error. States:

```
queued → claimed → running → succeeded | failed | timed_out | cancelled | attention
queued → skipped            (job already running, or paused when the claim happens)
claimed → blocked_config    (preflight failed; nothing launched)
running → unknown           (daemon restarted and the pid is gone; never re-run)
```

Environment given to every run: `AGENTCRON_JOB`, `AGENTCRON_RUN_ID`,
`AGENTCRON_SCHEDULED_FOR`, `AGENTCRON_WORKDIR`, `AGENTCRON_LAST_OUTPUT` (path or empty),
`AGENTCRON_MEMORY_NOTE` (when memory is on).

## 6. CLI contract

```
agentcron init                                  create config, jobs and data dirs (idempotent)
agentcron add <name> --schedule S --runner R [--prompt T | --prompt-file F] [--repo P] [--base B]
          [--timezone Z] [--arg A]... [--command C]... [--timeout D] [--expect RE] [--memory | --no-memory] [--dry-run]
agentcron ls [--json]                           jobs with schedule, next fire, last state
agentcron show <name> [--json]                  job, state, next 5 fires, last 5 runs
agentcron rm <name> [--dry-run]                 removes the file; runs stay in the ledger
agentcron pause|resume <name>
agentcron approve <name>                        approve an agent-created job
agentcron next [--schedule S --timezone Z] [--count N] [--json]
agentcron run <name> [--now] [--wait]           enqueue (daemon runs it) or run inline
agentcron runs [<name>] [--state S] [--limit N] [--json]
agentcron logs <run-id> [--stderr] [--follow]
agentcron cancel <run-id>
agentcron worktrees [--json] | prune [--dry-run]
agentcron memory <name>                         print the job's memory note via mdkb
agentcron daemon                                foreground scheduler
agentcron tick                                  one scheduling pass, then exit
agentcron service install|uninstall|status [--dry-run]
agentcron mcp [--allow-write]
agentcron tui
agentcron doctor [--json]                       runners, git, mdkb, service, lock, db
agentcron version
```

Runner argv (verify flags against the installed CLI's `--help` in M2 and pin them in a
test):
- `claude`: `claude -p <preamble+prompt> --output-format text [--mcp-config runs/<id>/mcp.json --strict-mcp-config] <args...>`, cwd = run worktree.
- `codex`: `codex exec -C <worktree> [-c mcp_servers.mdkb.command=... -c mcp_servers.mdkb.args=[...]] <args...> <preamble+prompt>`.
- `command`: `<command...>` with preamble+prompt on stdin, cwd = run worktree.

Exit codes: 0 ok; 1 runtime error; 2 usage or not found; 3 not initialised; 4 locked
(another daemon or tick holds the lock); 5 refused (policy: write without `--allow-write`,
scheduling from inside a run, unapproved job); 6 `--wait` run ended in a non-success
state. Global flags: `--json` on every read, `--quiet`, `--config`, `--data-dir`.
Env: `AGENTCRON_CONFIG_DIR`, `AGENTCRON_DATA_DIR`. Config precedence: flag > env > file >
default.

MCP tools: reads `list_jobs`, `get_job`, `list_runs`, `get_run_output` (tail, bounded),
`next_fires`; writes (only with `--allow-write`) `create_job`, `pause_job`, `resume_job`,
`run_job` (enqueue). Writes are refused with a tool error when `AGENTCRON_RUN_ID` is set,
unless `allow_agent_scheduling = true` in config. Jobs created through MCP start
unapproved and never fire until `approve` (CLI or TUI).

## 7. Milestones

Each milestone is one GitHub issue, one branch, one PR, one worker thread in its own
worktree. `Depends on` tells the orchestrator which milestones can run in parallel.

| M | Deliverable | Depends on | Done when |
|---|---|---|---|
| M0 | Scaffold: module, cobra root, `version`/`init`/`doctor` stub, config + paths, store with migrations, clock seam, fake runner binaries, CI, `accept.sh` | – | §3 rows 1–3; `accept.sh` green on ubuntu and macos CI |
| M1 | Jobs and schedules: TOML files, validation, add/ls/show/rm/pause/resume/next | M0 | rows 4–5; goldens for `ls`, `show`, `next` incl. DST cases |
| M2 | Runner and ledger: preflight, worktrees, adapters, preamble, timeouts, cancel, logs, expect, `run --now`, runs/logs/worktrees/prune | M1 | rows 6–11; fake runners prove every terminal state |
| M3 | Daemon: lock, tick loop, claims, concurrency, no overlap, catch-up, reconcile, retention, `tick`, service units | M2 | rows 12–15; fake-clock tests for every fire and restart rule |
| M4 | Memory via mdkb: detection, note bootstrap, MCP injection for claude/codex, preamble lines, `memory`, doctor | M2 | row 16; fake runner sees the mdkb MCP config and note id |
| M5 | MCP server: read and write tools, recursion guard, approval flow | M3 | row 17; stdio smoke with stdin held open |
| M6 | TUI | M3 | row 18; model tests on key handling and refresh |
| M7 | Release: workflow, tarballs, checksums, formula, README quick start | M4, M5, M6 | row 19; tag dry-run builds four archives |

Parallel: M3 ∥ M4 after M2 (merge M3 first). M5 ∥ M6 after M3 (merge M5 first; both
only add one line to the cobra root).

## 8. Acceptance

Tests and accept.sh preview time with the hidden read-only flag `--now <RFC3339>`; write commands refuse it.

`scripts/accept.sh` is the acceptance. It builds, runs the gate (race tests, gofmt, vet,
staticcheck), isolates HOME/XDG/git config in a scratch dir, puts the fake runners first
on PATH (and asserts they are first), seeds `testdata/jobs/`, runs every v1 read command
with `--json` and diffs against `testdata/golden/`, then live checks goldens cannot
express: exit codes, `run --now` against each fake runner outcome, a timeout, a killed
daemon leaving `unknown`, a second daemon refused with exit 4, MCP writes refused without
`--allow-write` and from inside a run. Each milestone extends the script. Worker,
reviewer and CI all run the same script; the orchestrator never retypes it.

## 9. Process

- One milestone per worker thread (`worker-brief`), draft PR, orchestrator reads the
  full diff and reruns `scripts/accept.sh`, independent `reviewer`, `pr-stress` when the
  diff touches process control, locking, worktrees or MCP, then `gh pr ready` and merge.
- Before the tag: `skeptic-review` on main from a fresh session per platform (Omarchy
  Linux, Mac Studio macOS); one `skeptic` issue per confirmed bug.
- Release through `ship`. Tests pin facts, not status prose.
- Tracking: GitHub issues are the truth; the Basecamp card for this repo
  (`project-board`) has one step per milestone.
- The orchestrator writes the worklog entry after each merge; workers report only.

## 10. Open questions

- Should a job be able to push branches or open PRs from its kept worktree, or is that
  left entirely to the runner's own permissions? v1: the runner's permissions decide.
- Desktop notification on `attention` is the first Later row worth pulling forward after
  a week of real use.
