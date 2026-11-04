---
name: skeptic-env
description: "How a skeptical reviewer builds, seeds and attacks agentcron. Read by skeptic-review; not a workflow on its own."
---

# skeptic-env — agentcron

Go CLI + daemon + stdio MCP + TUI that launches agent CLIs on a schedule. Job files are
truth; SQLite holds state and the run ledger.

## Stack and commands

```bash
go build -o /tmp/agentcron-skeptic ./cmd/agentcron
go test -race ./...
test -z "$(gofmt -l .)" && go vet ./... && staticcheck ./...
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o /tmp/agentcron-cross ./cmd/agentcron
# Build/test before isolating HOME (which also moves Go's caches).
scratch=$(mktemp -d)
export HOME="$scratch/home" XDG_CONFIG_HOME="$scratch/config" XDG_DATA_HOME="$scratch/data"
export GIT_CONFIG_GLOBAL="$scratch/gitconfig" GIT_CONFIG_NOSYSTEM=1 MDKB_ROOT="$scratch/kb"
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_DATA_HOME"
# Put the fakes from testdata/fakes first on PATH and assert it:
# [ "$(command -v claude)" = "$scratch/bin/claude" ] || exit 1
/tmp/agentcron-skeptic init
```

Never use the reviewer's real config, jobs, data dir, KB or service units. Never run the
real `claude` or `codex` against a paid account. Kill every daemon, tick and child you
start; check `pgrep -f agentcron-skeptic` is empty before you finish.

## Platforms

| Platform | Ships | Review on |
|---|---|---|
| Linux amd64/arm64 | yes | Omarchy (systemd user units) |
| macOS amd64/arm64 | yes | Mac Studio (launchd agents) |
| Windows | no | out of scope |

## What to attack

- Schedules: DST gaps and repeats, `at` in the past, Feb 29, timezone typos, `every 0s`.
- Daemon: two daemons, daemon + tick, kill -9 mid-run then restart (expect `unknown`,
  no re-run), clock jumps, catch-up after a long sleep, concurrency limit under a burst.
- Runs: child that ignores SIGTERM, grandchildren that outlive the child, huge stdout,
  binary output, a runner missing from PATH, a repo with no origin, a dirty base.
- Worktrees: cleanup of clean vs dirty vs ahead, prune with a run in flight, a deleted
  repo, branch name collisions.
- Job files: hand-edited while the daemon runs, invalid TOML, name/file mismatch,
  symlinks and traversal in `prompt_file`.
- MCP: writes without `--allow-write`, writes from inside a run, unapproved jobs firing,
  bounded `get_run_output`; hold stdin open (SDK cancels pending requests at EOF).
- Service: install twice, uninstall with the daemon running, PATH captured correctly.

## Traps

Fakes must be first on PATH or real CLIs get called. `/tmp` on Omarchy is a small tmpfs.

## Earlier rounds

None yet.
