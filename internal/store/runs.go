package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base32"
	"errors"
	"fmt"
	"time"
)

type Run struct {
	ID           string     `json:"id"`
	Job          string     `json:"job"`
	Trigger      string     `json:"trigger"`
	ScheduledFor time.Time  `json:"scheduled_for"`
	State        string     `json:"state"`
	PID          *int       `json:"pid"`
	OwnerPID     *int       `json:"owner_pid"`
	ProcStart    *time.Time `json:"proc_start"`
	ExitCode     *int       `json:"exit_code"`
	StartedAt    *time.Time `json:"started_at"`
	EndedAt      *time.Time `json:"ended_at"`
	Worktree     string     `json:"worktree"`
	Kept         bool       `json:"kept"`
	Error        string     `json:"error"`
}

func NewRun(job, trigger string, at time.Time) (Run, error) {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Run{}, err
	}
	return Run{ID: "r_" + at.UTC().Format("20060102T150405Z") + "_" + base32.StdEncoding.EncodeToString(b[:])[:4], Job: job, Trigger: trigger, ScheduledFor: at.UTC(), State: "queued"}, nil
}
func Terminal(state string) bool {
	switch state {
	case "succeeded", "failed", "timed_out", "cancelled", "attention", "skipped", "blocked_config", "unknown":
		return true
	}
	return false
}
func (s *Store) CreateRun(ctx context.Context, r Run) error {
	return s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		_, err := tx.Exec(ctx, "INSERT INTO runs(id,job,trigger,scheduled_for,state) VALUES(?,?,?,?,'queued')", r.ID, r.Job, r.Trigger, r.ScheduledFor.UTC().Format(time.RFC3339Nano))
		return err
	})
}
func (s *Store) transition(ctx context.Context, id, from, set string, args ...any) error {
	return s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		args = append(args, id, from)
		n, err := tx.Exec(ctx, "UPDATE runs SET "+set+" WHERE id=? AND state=?", args...)
		if err == nil && n != 1 {
			return fmt.Errorf("illegal run transition: %s from %s", id, from)
		}
		return err
	})
}
func (s *Store) ClaimRun(ctx context.Context, id string) error {
	return s.transition(ctx, id, "queued", "state='claimed'")
}
func (s *Store) StartRun(ctx context.Context, id string, pid int, at time.Time, worktree string) error {
	return s.StartOwnedRun(ctx, Run{ID: id, PID: &pid, StartedAt: &at, Worktree: worktree})
}
func (s *Store) StartOwnedRun(ctx context.Context, r Run) error {
	var proc any
	if r.ProcStart != nil {
		proc = r.ProcStart.UTC().Format(time.RFC3339Nano)
	}
	return s.transition(ctx, r.ID, "claimed", "state='running',pid=?,started_at=?,worktree=?,owner_pid=?,proc_start=?", r.PID, r.StartedAt.UTC().Format(time.RFC3339Nano), r.Worktree, r.OwnerPID, proc)
}
func (s *Store) FinishClaimedRun(ctx context.Context, r Run) error {
	if r.State != "failed" && r.State != "timed_out" {
		return fmt.Errorf("invalid claimed outcome")
	}
	return s.finishRun(ctx, r, "claimed")
}
func (s *Store) FinishRun(ctx context.Context, r Run) error {
	from := "running"
	switch r.State {
	case "skipped":
		from = "queued"
	case "blocked_config":
		from = "claimed"
	case "succeeded", "failed", "timed_out", "cancelled", "attention", "unknown":
	default:
		return fmt.Errorf("invalid terminal state %q", r.State)
	}
	return s.finishRun(ctx, r, from)
}
func (s *Store) finishRun(ctx context.Context, r Run, from string) error {
	if r.EndedAt == nil {
		return fmt.Errorf("ended_at required")
	}
	return s.transition(ctx, r.ID, from, "state=?,exit_code=?,ended_at=?,error=?,kept=?,worktree=?", r.State, r.ExitCode, r.EndedAt.UTC().Format(time.RFC3339Nano), r.Error, r.Kept, r.Worktree)
}

const runColumns = "id,job,trigger,scheduled_for,state,pid,exit_code,started_at,ended_at,COALESCE(worktree,''),kept,COALESCE(error,''),owner_pid,proc_start"

func scanRun(row interface{ Scan(...any) error }) (Run, error) {
	var r Run
	var scheduled string
	var started, ended, proc sql.NullString
	err := row.Scan(&r.ID, &r.Job, &r.Trigger, &scheduled, &r.State, &r.PID, &r.ExitCode, &started, &ended, &r.Worktree, &r.Kept, &r.Error, &r.OwnerPID, &proc)
	if err != nil {
		return r, err
	}
	if r.ScheduledFor, err = time.Parse(time.RFC3339Nano, scheduled); err != nil {
		return r, err
	}
	for _, v := range []struct {
		src sql.NullString
		dst **time.Time
	}{{started, &r.StartedAt}, {ended, &r.EndedAt}, {proc, &r.ProcStart}} {
		if v.src.Valid {
			t, e := time.Parse(time.RFC3339Nano, v.src.String)
			if e != nil {
				return r, e
			}
			t = t.UTC()
			*v.dst = &t
		}
	}
	r.ScheduledFor = r.ScheduledFor.UTC()
	return r, nil
}

var ErrRunNotFound = errors.New("run not found")

func (s *Store) GetRun(ctx context.Context, id string) (Run, error) {
	r, err := scanRun(s.db.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE id=?", id))
	if errors.Is(err, sql.ErrNoRows) {
		err = fmt.Errorf("%w: %s", ErrRunNotFound, id)
	}
	return r, err
}
func (s *Store) ListRuns(ctx context.Context, job, state string, limit int) ([]Run, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT "+runColumns+" FROM runs WHERE (?='' OR job=?) AND (?='' OR state=?) ORDER BY rowid DESC LIMIT ?", job, job, state, state, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Run{}
	for rows.Next() {
		r, e := scanRun(rows)
		if e != nil {
			return nil, e
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
func (s *Store) ClearKept(ctx context.Context, id string) error {
	return s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		n, e := tx.Exec(ctx, "UPDATE runs SET kept=0 WHERE id=? AND kept=1 AND state NOT IN ('queued','claimed','running')", id)
		if e == nil && n != 1 {
			return fmt.Errorf("run is not terminal and kept")
		}
		return e
	})
}

// PreviousRun finds terminal history independently of enqueue order.
func (s *Store) PreviousRun(ctx context.Context, r Run) (*Run, error) {
	previous, err := scanRun(s.db.QueryRowContext(ctx, "SELECT "+runColumns+" FROM runs WHERE state NOT IN ('queued','claimed','running') AND job=? AND id<>? ORDER BY rowid DESC LIMIT 1", r.Job, r.ID))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &previous, nil
}
