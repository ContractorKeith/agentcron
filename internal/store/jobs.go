package store

import (
	"context"
	"database/sql"
	"time"
)

type JobState struct {
	Fired       bool       `json:"-"`
	ScheduleKey string     `json:"-"`
	Name        string     `json:"name"`
	Paused      bool       `json:"paused"`
	Approved    bool       `json:"approved"`
	NextFire    *time.Time `json:"next_fire"`
	LastRunID   string     `json:"last_run_id"`
	UpdatedAt   time.Time  `json:"updated_at"`
	AnchorAt    time.Time  `json:"anchor_at"`
	Orphaned    bool       `json:"orphaned"`
}

func (s *Store) JobStates(ctx context.Context) (map[string]JobState, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT name,paused,approved,next_fire,last_run_id,updated_at,anchor_at,orphaned,schedule_key,fired FROM job_state")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := map[string]JobState{}
	for rows.Next() {
		var st JobState
		var next, last, anchor sql.NullString
		var updated string
		if err := rows.Scan(&st.Name, &st.Paused, &st.Approved, &next, &last, &updated, &anchor, &st.Orphaned, &st.ScheduleKey, &st.Fired); err != nil {
			return nil, err
		}
		st.LastRunID = last.String
		if st.UpdatedAt, err = time.Parse(time.RFC3339Nano, updated); err != nil {
			return nil, err
		}
		if anchor.Valid {
			if st.AnchorAt, err = time.Parse(time.RFC3339Nano, anchor.String); err != nil {
				return nil, err
			}
		}
		if next.Valid {
			t, e := time.Parse(time.RFC3339Nano, next.String)
			if e != nil {
				return nil, e
			}
			t = t.UTC()
			st.NextFire = &t
		}
		st.UpdatedAt = st.UpdatedAt.UTC()
		st.AnchorAt = st.AnchorAt.UTC()
		states[st.Name] = st
	}
	return states, rows.Err()
}

func (s *Store) PutJobState(ctx context.Context, st JobState) error {
	var next any
	if st.NextFire != nil {
		next = st.NextFire.UTC().Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO job_state
 (name,paused,approved,next_fire,last_run_id,updated_at,anchor_at,orphaned,schedule_key,fired) VALUES (?,?,?,?,?,?,?,?,?,?)
 ON CONFLICT(name) DO UPDATE SET paused=excluded.paused,approved=excluded.approved,
 next_fire=excluded.next_fire,last_run_id=excluded.last_run_id,updated_at=excluded.updated_at,
 anchor_at=excluded.anchor_at,orphaned=excluded.orphaned,schedule_key=excluded.schedule_key,fired=excluded.fired`,
		st.Name, st.Paused, st.Approved, next, st.LastRunID, st.UpdatedAt.UTC().Format(time.RFC3339Nano), st.AnchorAt.UTC().Format(time.RFC3339Nano), st.Orphaned, st.ScheduleKey, st.Fired)
	return err
}
