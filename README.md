# agentcron

Schedule recurring agent work on your own Mac or Linux machine. One Go binary runs a daemon, CLI, stdio MCP server and TUI: jobs fire on cron-style schedules, launch Claude Code, Codex or any command in a fresh git worktree (or scratch directory), and record their output in a SQLite run ledger. Optional [mdkb](https://github.com/ContractorKeith/mdkb) memory notes let each job build on its previous work. Agents can propose jobs over MCP; a human approves them before they run.

**Status:** pre-release; see [the plan](docs/PLAN.md), [changelog](CHANGELOG.md) and [glossary](CONTEXT.md). v0.1.0 release candidate; skeptic review pending. MIT licensed.

## Install

After v0.1.0 and its tap formula are published:

```bash
brew install contractorkeith/tap/agentcron
```

From source (Go 1.27.3 or newer):

```bash
go install github.com/ContractorKeith/agentcron/cmd/agentcron@latest
export PATH="$(go env GOPATH)/bin:$PATH"
```

Source installs report `dev`; release binaries report the release tag. Or download the matching macOS/Linux amd64/arm64 tarball and `checksums.txt` from [GitHub Releases](https://github.com/ContractorKeith/agentcron/releases). In the download directory, verify the archive before extracting:

```bash
# Example: Linux amd64; choose the filename for your platform.
grep '  agentcron_v0.1.0_linux_amd64.tar.gz$' checksums.txt | sha256sum -c -
tar -xzf agentcron_v0.1.0_linux_amd64.tar.gz
# Place agentcron in a directory on your PATH.
```

On macOS, use `shasum -a 256 -c -` instead of `sha256sum -c -` and the matching `darwin` archive. Tarball binaries are unsigned, so Gatekeeper may block them. Prefer Homebrew or `go install`. After verifying the checksum, remove download quarantine with `xattr -d com.apple.quarantine agentcron` if needed. Git on PATH is required for repository worktrees.

## Quick start (60 seconds)

Run this in a shell with agentcron and Claude Code on PATH. It creates a scratch HOME, config and data directory; the subshell restores your environment afterward. The `cat` job needs no model or mdkb. MCP registration is local to the scratch directory. Remove the printed scratch directory when finished.

<!-- quickstart:start -->
```bash
(
  example_root="$(mktemp -d)"
  export HOME="$example_root/home"
  export XDG_CONFIG_HOME="$example_root/config" XDG_DATA_HOME="$example_root/data"
  export XDG_CACHE_HOME="$example_root/cache" MDKB_ROOT="$example_root/kb"
  export GIT_CONFIG_GLOBAL="$example_root/gitconfig" GIT_CONFIG_NOSYSTEM=1
  unset AGENTCRON_CONFIG_DIR AGENTCRON_DATA_DIR AGENTCRON_RUN_ID
  mkdir -p "$HOME"
  cd "$example_root"
  agentcron init
  agentcron add hello --schedule 'every 1h' --runner command --command cat \
    --prompt 'Hello from agentcron' --no-memory
  run_id="$(agentcron run hello --now)"
  agentcron runs hello
  agentcron logs "$run_id"
  agentcron service install --dry-run
  claude mcp add --scope local agentcron -- "$(command -v agentcron)" mcp
  printf 'Scratch directory: %s\n' "$example_root"
)
```
<!-- quickstart:end -->

For regular use, run `agentcron init` in your normal environment. `agentcron daemon` runs the scheduler in the foreground; `agentcron service install` installs and starts a systemd user service on Linux or a launchd agent on macOS, capturing the current PATH. `agentcron run hello` queues a run for the daemon; `--now` runs inline and waits. `agentcron tick` performs one scheduling pass for external cron. Use `agentcron doctor` to check configuration and runner availability. Read commands support `--json`.

## Writing jobs

Each job is a TOML file at `~/.config/agentcron/jobs/<name>.toml` by default. `XDG_CONFIG_HOME` or `AGENTCRON_CONFIG_DIR` can select a different location; data defaults to `~/.local/share/agentcron`. Names use lowercase letters, digits and hyphens, starting with a letter or digit, up to 63 characters. `agentcron add` validates and writes a job; `--dry-run` previews its TOML.

This Claude job triages GitHub issues in a repository you supply:

```toml
# jobs/morning-issues.toml
schedule = "0 7 * * 1-5"
timezone = "America/New_York"
runner = "claude"
prompt = "List new GitHub issues. Summarize the ones needing attention."
repo = "~/Projects/your-repo"
base = "HEAD"
args = ["--permission-mode", "default", "--allowedTools", "Bash(gh issue list *)", "Bash(gh issue view *)"]
timeout = "15m"
memory = false
catchup = "one"
created_by = "cli"
```

A Codex job can inspect the same repository with its read-only sandbox:

```toml
# jobs/review.toml
schedule = "@daily"
runner = "codex"
prompt = "Read the source and report one concrete improvement. Do not edit files."
repo = "~/Projects/your-repo"
args = ["--sandbox", "read-only"]
timeout = "15m"
memory = false
catchup = "none"
created_by = "cli"
```

| Field | Meaning |
|---|---|
| `schedule` | Required: five-field cron, `every 2h`, `at <RFC3339>` or a macro such as `@daily`. |
| `timezone` | IANA zone; defaults to local time. Missing DST times are skipped; repeated times fire once. |
| `runner` | Required: `claude`, `codex` or `command`. The executable must be on PATH. |
| `prompt` / `prompt_file` | Exactly one; prompt files must stay inside the jobs directory, including through symlinks. |
| `repo` / `base` | Optional repository and base ref. Base defaults to origin's HEAD, otherwise HEAD. Without a repo each run gets a scratch directory. |
| `args` | Extra argv for Claude/Codex; CLI spelling is repeatable `--arg`. |
| `command` | For the command runner: argv such as `["cat"]`; the preamble and prompt arrive on stdin. CLI spelling is repeatable `--command`. |
| `timeout` | Positive duration, default `30m`, covering setup and the run. |
| `expect` | Optional stdout regex; exit 0 without a match becomes `attention`. |
| `memory` | CLI `add` defaults to mdkb presence; `--memory` / `--no-memory` override. Hand-written TOML defaults to true. |
| `catchup` | `one` (default) catches up one missed fire; `none` skips missed fires. |
| `created_by` | `cli` or `mcp`; agentcron sets provenance when creating jobs. MCP jobs require human approval. |

Runs get a preamble with the job, run id, fire time, previous state and output path, plus the memory note when enabled. Repository runs use branches `agentcron/<job>/<run-id>`; clean worktrees are removed, dirty or ahead worktrees are kept. Inspect with `agentcron worktrees`; preview cleanup with `agentcron prune --dry-run`. Pause/resume with `agentcron pause <name>` / `agentcron resume <name>`.

`prune` keeps dirty or ahead worktrees and lists the files/commits `--force` would destroy; `prune --dry-run` lists all candidates.

## Memory with mdkb

Install and initialize [mdkb](https://github.com/ContractorKeith/mdkb) separately. With memory enabled, agentcron creates or reuses `agentcron/<job>` in the KB and injects mdkb's MCP server into Claude/Codex runs. The prompt tells the runner which note to read and update. `agentcron memory <name>` prints the note. Missing mdkb degrades to a run without memory and appears in `doctor` warnings.

Set `mdkb_path` and `mdkb_root` in agentcron's `config.toml` to override the executable and KB root. An empty root preserves mdkb's own precedence (`MDKB_ROOT`, then its config/default). A job's explicit mdkb server configuration takes precedence over injection.

Service install captures and prints `PATH` and, when set, `MDKB_ROOT`; reinstall after changing either.

On Linux, run `loginctl enable-linger $USER` to keep the user service running after logout.

## Safety defaults

An `unknown` outcome after a daemon crash is never automatically re-run. Preflight failures become `blocked_config` and launch nothing. MCP-created jobs start paused and unapproved; a human must run `agentcron approve <name>` or approve in the TUI. MCP write tools require `--allow-write`. CLI, MCP and TUI scheduling writes refuse calls inside a run unless `allow_agent_scheduling = true` is set in config. Approval is refused inside a run even when that setting is enabled. The recursion guard stops accidental and casual recursion; an agent with unrestricted shell can unset `AGENTCRON_RUN_ID`, so runner permissions remain the boundary.

Each run has its own process group. Timeout and cancellation signal the group with SIGTERM, then SIGKILL after a grace period. Descendants that leave the run's process group (`setsid`, `setpgid`, or shell job control) are not torn down; OS sandboxing is Later. On Linux, stopping the service still kills them through the unit's cgroup. Runner permissions control access outside the worktree. Hand-written job files are trusted local configuration: prevent runners from writing the config directory and check `doctor` warnings when jobs live inside a job repository.

Agentcron preserves the job's runner permissions with one scoped allowance: when it injects mdkb into Claude, it adds `--allowedTools mcp__mdkb__*` to auto-approve that server's tools. A job supplying its own `--mcp-config` receives no injected server or allowance. See [ADR 0003](docs/adr/0003-unattended-safety-defaults.md).

## MCP

The quick start registers the read-only stdio server. Add `--allow-write` after `mcp` to expose job creation, pause/resume and enqueue tools. Approval remains a human action. See [MCP tools, configuration and approval](docs/mcp.md).

## TUI

Run `agentcron tui` in an interactive terminal to browse jobs, runs and live logs, run jobs now, pause/resume and approve proposed jobs. The on-screen help lists keys.

For release preparation, see [Homebrew publishing](packaging/homebrew/README.md). Run `bash scripts/accept.sh` for the full local gate and `bash scripts/release-dry-run.sh v0.0.0-dryrun` to verify four temporary release archives without publishing.
