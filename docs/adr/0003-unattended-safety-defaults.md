# 3. Unattended runs fail safe

An `unknown` run is never re-run, preflight failures launch nothing, runs keep the job's own permissions except the scoped mdkb allowance described below, jobs created through MCP do not fire until a human approves them, and MCP writes are refused inside a scheduled run unless `allow_agent_scheduling` is set. This costs some convenience for agent-created jobs and buys no runaway self-scheduling and no double-spent runs.

When agentcron injects mdkb into a Claude run, it adds
`--allowedTools mcp__mdkb__*`. This is the one permission allowance agentcron
adds: only tools from the injected `mdkb` server are auto-approved. Other tools
retain the job's permissions. A job's own `--mcp-config` receives no added
allowance or injected server. Codex overrides for `mcp_servers.mdkb` likewise
preserve the job's own server configuration.

The server-scoped wildcard follows the [Claude Code MCP permission rules](https://code.claude.com/docs/en/permissions#mcp).
