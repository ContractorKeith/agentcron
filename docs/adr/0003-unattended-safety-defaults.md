# 3. Unattended runs fail safe

An `unknown` run is never re-run, preflight failures launch nothing, runs never get permissions beyond the job's own `args`, jobs created through MCP do not fire until a human approves them, and MCP writes are refused inside a scheduled run unless `allow_agent_scheduling` is set. This costs some convenience for agent-created jobs and buys no runaway self-scheduling and no double-spent runs.
