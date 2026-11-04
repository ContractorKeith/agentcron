package store

import (
	"context"
	"os"
	"time"
)

func (s *Store) ClaimOwned(ctx context.Context, id string, at time.Time, ownerStart time.Time) error {
	return s.transition(ctx, id, "queued", "state='claimed',owner_pid=?,started_at=?,owner_start=?", os.Getpid(), at.UTC().Format(time.RFC3339Nano), ownerStart.UTC().Format(time.RFC3339Nano))
}
func (s *Store) Unknown(ctx context.Context, r Run) error {
	from := r.State
	r.State = "unknown"
	return s.finishRun(ctx, r, from)
}
func (s *Store) DeleteTerminal(ctx context.Context, id string) (bool, error) {
	var deleted int64
	err := s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		var err error
		deleted, err = tx.Exec(ctx, "DELETE FROM runs WHERE id=? AND kept=0 AND state NOT IN ('queued','claimed','running')", id)
		return err
	})
	return err == nil && deleted == 1, err
}
func (s *Store) SetDaemonStopped(ctx context.Context, id string) error {
	return s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		_, e := tx.Exec(ctx, "UPDATE runs SET state='cancelled',error='daemon stopped' WHERE id=? AND state='cancelled'", id)
		return e
	})
}

// SyncDefinition preserves a cursor advanced concurrently by the scheduler.
func (s *Store) SyncDefinition(ctx context.Context, st JobState) error {
	var next any
	if st.NextFire != nil {
		next = st.NextFire.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO job_state
 (name,paused,approved,next_fire,last_run_id,updated_at,anchor_at,orphaned,schedule_key,fired) VALUES(?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(name) DO UPDATE SET
 next_fire=CASE WHEN job_state.schedule_key=excluded.schedule_key AND job_state.orphaned=0 AND excluded.orphaned=0 THEN job_state.next_fire ELSE excluded.next_fire END,
 fired=CASE WHEN job_state.schedule_key=excluded.schedule_key THEN job_state.fired ELSE excluded.fired END,
 last_run_id=excluded.last_run_id,updated_at=excluded.updated_at,anchor_at=excluded.anchor_at,
 orphaned=excluded.orphaned,schedule_key=excluded.schedule_key`, st.Name, st.Paused, st.Approved, next, st.LastRunID, st.UpdatedAt.UTC().Format(time.RFC3339Nano), st.AnchorAt.UTC().Format(time.RFC3339Nano), st.Orphaned, st.ScheduleKey, st.Fired)
	return err
}
func (s *Store) AdvanceCursor(ctx context.Context, st JobState) error {
	var next any
	if st.NextFire != nil {
		next = st.NextFire.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, "UPDATE job_state SET next_fire=?,fired=?,updated_at=? WHERE name=? AND schedule_key=?", next, st.Fired, st.UpdatedAt.UTC().Format(time.RFC3339Nano), st.Name, st.ScheduleKey)
	return err
}
