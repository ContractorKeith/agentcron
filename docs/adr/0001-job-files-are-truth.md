# 1. Job definitions are TOML files; SQLite holds state and the ledger

Jobs live as one TOML file each in `~/.config/agentcron/jobs/` so people can read, edit, diff and version them; the database only holds job state (paused, approved, next fire) and the run ledger. Losing the database loses history, never jobs; the cost is reconciling file edits on every tick.
