CREATE TABLE job_state (
 name TEXT PRIMARY KEY,
 paused INTEGER NOT NULL DEFAULT 0 CHECK (paused IN (0, 1)),
 approved INTEGER NOT NULL DEFAULT 0 CHECK (approved IN (0, 1)),
 next_fire TEXT,
 last_run_id TEXT,
 updated_at TEXT NOT NULL
);

CREATE TABLE runs (
 id TEXT PRIMARY KEY,
 job TEXT NOT NULL,
 trigger TEXT NOT NULL CHECK (trigger IN ('schedule', 'manual', 'catchup', 'mcp')),
 scheduled_for TEXT NOT NULL,
 state TEXT NOT NULL CHECK (state IN (
  'queued', 'claimed', 'running', 'succeeded', 'failed', 'timed_out',
  'cancelled', 'attention', 'skipped', 'blocked_config', 'unknown'
 )),
 pid INTEGER,
 exit_code INTEGER,
 started_at TEXT,
 ended_at TEXT,
 worktree TEXT,
 kept INTEGER NOT NULL DEFAULT 0 CHECK (kept IN (0, 1)),
 error TEXT
);
CREATE UNIQUE INDEX runs_fire ON runs (job, scheduled_for, trigger)
 WHERE trigger IN ('schedule', 'catchup');
CREATE INDEX runs_state ON runs (state);
CREATE INDEX runs_job_started_at ON runs (job, started_at);
