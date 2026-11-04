package store

import (
	"context"
	"strings"
	"testing"
)

func TestLedgerQueriesUseIndexes(t *testing.T) {
	s, err := Open(context.Background(), scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, tc := range []struct{ query, index string }{
		{"SELECT id FROM runs WHERE state='queued'", "runs_state"},
		{"SELECT id FROM runs WHERE job='job' ORDER BY started_at", "runs_job_started_at"},
	} {
		t.Run(tc.index, func(t *testing.T) {
			rows, err := s.db.Query("EXPLAIN QUERY PLAN " + tc.query)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var plan strings.Builder
			for rows.Next() {
				var id, parent, unused int
				var detail string
				if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
					t.Fatal(err)
				}
				plan.WriteString(detail)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(plan.String(), tc.index) {
				t.Fatalf("query did not use %s: %s", tc.index, plan.String())
			}
			if strings.Contains(plan.String(), "TEMP B-TREE") {
				t.Fatalf("query sorted outside the index: %s", plan.String())
			}
		})
	}
}

func TestWALConfiguredForEveryConnection(t *testing.T) {
	s, err := Open(context.Background(), scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	// Each operation gets a fresh connection; changing the persistent mode must not
	// bypass the WAL setup when the pool next opens one.
	s.db.SetMaxIdleConns(0)
	if _, err := s.db.Exec("PRAGMA journal_mode=DELETE"); err != nil {
		t.Fatal(err)
	}
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("new pooled connection uses %s, want wal", mode)
	}
}
