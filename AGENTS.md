# agentcron

A local scheduler for recurring agent work: one Go binary with a CLI, a daemon, a stdio MCP
server and a TUI. Jobs (one TOML file each) fire on cron-style schedules, run `claude`,
`codex` or any command in a fresh git worktree with the job's own permissions, land in a
SQLite run ledger with their output, and get a per-job memory note in a shared mdkb
knowledge base. "Done" for v1 is every row marked v1 in `docs/PLAN.md` §3 passing
`scripts/accept.sh` on macOS and Linux.

## Status
M3 — daemon, tick and service units: done (#4).

## Commands
```bash
bash scripts/accept.sh          # the acceptance (lands in M0); CI runs this
go build ./cmd/agentcron && ./agentcron version
go test -race ./...
test -z "$(gofmt -l .)" && go vet ./... && staticcheck ./...
```

## Pointers
- `docs/PLAN.md` — goal, feature matrix, architecture, data model, CLI contract, milestones, acceptance.
- `CONTEXT.md` — glossary; use its words in code, docs and issues.
- `docs/adr/` — job files are truth, memory is mdkb, unattended safety defaults.
- Session logs: `~/Projects/worklog/projects/agentcron.md` (not in this repo).

## Conventions & Gotchas
- Only `internal/store` imports `database/sql`; only `internal/jobs` touches job files;
  `cmd/`, `internal/mcpserver` and `internal/tui` go through `internal/app`.
- No `time.Now` outside `internal/clock`; scheduler tests use the fake clock.
- Tests never run real `claude`, `codex` or `mdkb`: build the fakes in `testdata/fakes/`
  and assert they are first on PATH before every run.
- Child processes get their own process group; timeouts and cancel signal the group, and
  every test reaps what it starts.
- Pure Go only (`modernc.org/sqlite`, no CGO). Never point tests at the real `~/kb`,
  `~/.config/agentcron` or `~/.local/share/agentcron`.
- `/tmp` on Omarchy is a small tmpfs; keep scratch data small and clean it up.
- Every read command has `--json`; every write has `--dry-run` where §6 says so.
- One milestone = one issue = one branch = one PR = one T3 thread in its own worktree.
- Commits: `<type>: <description>`, author @ContractorKeith only.

## Tracking
GitHub issues (label `milestone`) are the truth. Basecamp "github" board card mirrors
milestones as steps (`project-board`). Skeptic findings get the `skeptic` label.

## Out of Scope
- Chat delivery channels, a built-in vector store, LLM calls of its own.
- Task graphs and spec-to-PR orchestration (that is agentpm).
- Containers and OS sandboxes in v1 (Later). Windows.
