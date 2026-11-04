ALTER TABLE job_state ADD COLUMN anchor_at TEXT;
ALTER TABLE job_state ADD COLUMN orphaned INTEGER NOT NULL DEFAULT 0 CHECK (orphaned IN (0, 1));
