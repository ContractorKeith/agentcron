# MCP stdio server

Initialize agentcron with `agentcron init`, then register its absolute binary path:

```sh
claude mcp add --scope local agentcron -- "$(command -v agentcron)" mcp
```

Append `--allow-write` after `mcp` to register write tools. Stdout belongs exclusively
to the MCP transport; diagnostics go to stderr. Global `--config` and `--data-dir`
select an isolated instance. An uninitialized instance exits 3 before transport startup.

| Tool | Arguments | Structured result |
|---|---|---|
| `list_jobs` | `{}` | CLI `ls --json` array of job views |
| `get_job` | `{name}` | CLI `show --json`: job, status, state, next five fires, last five runs |
| `list_runs` | `{job?, state?, limit?}` | CLI `runs --json` array; default 50, range 1–200 |
| `get_run_output` | `{id, stream, tail_bytes?}` | `{id, stream, output, truncated}`; stream `stdout` or `stderr`; default 16384 bytes, range 1–262144 |
| `next_fires` | `{schedule?, timezone?, count?}` | CLI `next --json` array; count per job defaults to 5, range 1–50; timezone requires schedule |
| `create_job` | `{name, schedule, runner, prompt?, prompt_file?, repo?, base?, timezone?, args?, command?, timeout?, expect?, memory?, no_memory?}` | Job detail, as `get_job` |
| `pause_job` | `{name}` | Updated job detail |
| `resume_job` | `{name}` | Updated job detail |
| `run_job` | `{name}` | Queued run, as CLI `runs --json` entries; `trigger = "mcp"` |

The last four tools require `--allow-write`. Creation uses the same fields, defaults
and validation as CLI `add`: exactly one prompt source, timeout defaults to `30m`,
memory defaults to mdkb presence, with mutually exclusive `memory` and `no_memory`
overrides; catch-up is `one`. Creation provenance is
always `mcp`. Arrays match CLI JSON directly, including empty arrays; text content
also contains the JSON. Output tails are bounded in bytes and set `truncated: true`
when earlier bytes were omitted. Missing logs on a known run return empty output.
Unknown jobs/runs, invalid fields or bounds, policy refusals and storage failures
return actionable tool errors (`isError: true`). Unregistered tools, including writes
without the flag, are rejected by the SDK as JSON-RPC errors.

## Approval and recursion guard

New MCP jobs start paused and unapproved. Resuming cannot approve them, and `run_job`
refuses unapproved jobs. A human runs `agentcron approve <name>` (or uses the TUI)
to approve and resume; repeated approval preserves subsequent pause choices.
Approval is never exposed through MCP. A daemon or `agentcron tick` runs the queue.

When `AGENTCRON_RUN_ID` is nonempty, every write tool refuses calls unless
`allow_agent_scheduling = true` is set in config.toml. This setting permits recursive
schedule changes but still requires human approval of newly created jobs.

Hand-written files remain trusted local configuration. Runner permissions must
prevent writes to the config directory; see [ADR 0003](adr/0003-unattended-safety-defaults.md).
`doctor` warns when jobs live inside a job's repository, and preflight blocks a run
whose worktree would contain the config directory, including through symlinks.

## Stdio smoke tests

Keep stdin open until responses arrive. An immediate EOF cancels SDK requests:

```sh
(printf '%s\n' \
  '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}' \
  '{"jsonrpc":"2.0","method":"notifications/initialized"}' \
  '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'; sleep 2) | agentcron mcp
```
