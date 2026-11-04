// Package store owns the SQLite ledger and its migrations.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

// Open creates or opens the ledger and applies migrations atomically.
func Open(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, true)
}

// OpenExisting inspects an existing ledger without creating or migrating it.
func OpenExisting(ctx context.Context, path string) (*Store, error) {
	return open(ctx, path, false)
}

func open(ctx context.Context, path string, create bool) (*Store, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	uri := url.URL{Scheme: "file", Path: absolute}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Set("_txlock", "immediate")
	if !create {
		q.Set("mode", "ro")
	}
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err = db.PingContext(ctx); err == nil && create {
		err = s.migrate(ctx)
	}
	if err != nil {
		_ = db.Close()
		var sqliteErr *sqlite.Error
		if errors.As(err, &sqliteErr) && (sqliteErr.Code()&255 == 11 || sqliteErr.Code()&255 == 26) {
			return nil, fmt.Errorf("open ledger %s: %w; move it aside and run agentcron init", absolute, err)
		}
		return nil, fmt.Errorf("open ledger %s: %w", absolute, err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_version").Scan(&version)
	return version, err
}

// ImmediateTx exposes queries on the connection holding the write reservation.
// The connection and any rows must not escape the callback.
type ImmediateTx struct{ tx *sql.Tx }

func (tx *ImmediateTx) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	result, err := tx.tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// WithImmediateTx reserves the writer before invoking fn, even before its first write.
func (s *Store) WithImmediateTx(ctx context.Context, fn func(*ImmediateTx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(&ImmediateTx{tx: tx}); err != nil {
		return err
	}
	return tx.Commit()
}
