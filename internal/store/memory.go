package store

import (
	"context"
	"database/sql"
)

func (s *Store) MemoryNote(ctx context.Context, job string) (string, error) {
	var note string
	err := s.db.QueryRowContext(ctx, "SELECT memory_note FROM job_state WHERE name=?", job).Scan(&note)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return note, err
}

func (s *Store) SetMemoryNote(ctx context.Context, job, note string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE job_state SET memory_note=? WHERE name=?", note, job)
	return err
}
