package store

import (
	"context"
	"errors"
	"fmt"
	"modernc.org/sqlite"
	"time"
)

// EnqueueFire reports actual insertion and retries primary-key collisions only.
func (s *Store) EnqueueFire(ctx context.Context, r Run) (Run, bool, error) {
	for range 8 {
		inserted, err := s.insertFire(ctx, r)
		if err == nil {
			return r, inserted, nil
		}
		var constraint *sqlite.Error
		if !errors.As(err, &constraint) || constraint.Code() != 1555 {
			return r, false, err
		}
		r, err = NewRun(r.Job, r.Trigger, r.ScheduledFor)
		if err != nil {
			return r, false, err
		}
	}
	return r, false, fmt.Errorf("could not allocate a unique run ID after 8 attempts")
}
func (s *Store) insertFire(ctx context.Context, r Run) (bool, error) {
	var count int64
	err := s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		var err error
		count, err = tx.Exec(ctx, `INSERT INTO runs(id,job,trigger,scheduled_for,state)
 SELECT ?,?,?,?,'queued' WHERE NOT EXISTS
 (SELECT 1 FROM runs WHERE job=? AND scheduled_for=? AND trigger IN ('schedule','catchup'))
 ON CONFLICT(job,scheduled_for,trigger) WHERE trigger IN ('schedule','catchup') DO NOTHING`, r.ID, r.Job, r.Trigger, r.ScheduledFor.UTC().Format(time.RFC3339Nano), r.Job, r.ScheduledFor.UTC().Format(time.RFC3339Nano))
		return err
	})
	return err == nil && count == 1, err
}
