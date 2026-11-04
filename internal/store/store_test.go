package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func scratchDB(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME"} {
		t.Setenv(key, dir)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig"))
	return filepath.Join(dir, "ledger.db")
}

func TestMigrateAndReopen(t *testing.T) {
	ctx := context.Background()
	path := scratchDB(t)
	for range 2 {
		s, err := Open(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		version, err := s.SchemaVersion(ctx)
		if err != nil || version != 4 {
			t.Fatalf("version %d: %v", version, err)
		}
		var journal string
		if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil || journal != "wal" {
			t.Fatalf("journal %s: %v", journal, err)
		}
		var timeout int
		if err := s.db.QueryRow("PRAGMA busy_timeout").Scan(&timeout); err != nil || timeout != 5000 {
			t.Fatalf("timeout %d: %v", timeout, err)
		}
		for table, want := range map[string][]string{
			"job_state": {"name", "paused", "approved", "next_fire", "last_run_id", "updated_at", "anchor_at", "orphaned", "schedule_key", "fired"},
			"runs":      {"id", "job", "trigger", "scheduled_for", "state", "pid", "exit_code", "started_at", "ended_at", "worktree", "kept", "error", "owner_pid", "proc_start"},
		} {
			rows, err := s.db.Query("SELECT * FROM " + table + " LIMIT 0")
			if err != nil {
				t.Fatal(err)
			}
			got, err := rows.Columns()
			_ = rows.Close()
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != len(want) {
				t.Fatalf("%s columns: %v", table, got)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("%s columns: %v", table, got)
				}
			}
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	s, err := OpenExisting(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM schema_version").Scan(&count); err != nil || count != 4 {
		t.Fatalf("migration rows %d: %v", count, err)
	}
}

func TestImmediateTxSerializesWriters(t *testing.T) {
	ctx := context.Background()
	path := scratchDB(t)
	a, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	held, release, entered, trying := make(chan struct{}), make(chan struct{}), make(chan struct{}), make(chan struct{})
	firstDone, secondDone := make(chan error, 1), make(chan error, 1)
	go func() {
		firstDone <- a.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
			close(held)
			<-release
			_, err := tx.Exec(ctx, "INSERT INTO job_state (name,updated_at) VALUES ('one','2026-11-04')")
			return err
		})
	}()
	<-held
	go func() {
		close(trying)
		secondDone <- b.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
			close(entered)
			// The first writer's row must already be committed when this callback starts.
			var count int
			if err := tx.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM job_state").Scan(&count); err != nil {
				return err
			}
			if count != 1 {
				return errors.New("first write not committed")
			}
			_, err := tx.Exec(ctx, "INSERT INTO job_state (name,updated_at) VALUES ('two','2026-11-04')")
			return err
		})
	}()
	<-trying
	early := false
	select {
	case <-entered:
		early = true
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	// Always join both goroutines, even when the lock assertion fails.
	if err := <-firstDone; err != nil {
		t.Error(err)
	}
	if err := <-secondDone; err != nil {
		t.Error(err)
	}
	if early {
		t.Fatal("second writer entered before reservation released")
	}
}

func TestRollbackAndUniqueFire(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	stop := errors.New("rollback")
	if err := s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO job_state (name,updated_at) VALUES ('rolled-back','now')"); err != nil {
			return err
		}
		return stop
	}); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM job_state").Scan(&count); err != nil || count != 0 {
		t.Fatalf("count %d: %v", count, err)
	}
	insert := "INSERT INTO runs (id,job,trigger,scheduled_for,state) VALUES (?,'job','schedule','2026-11-04','queued')"
	if _, err := s.db.Exec(insert, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(insert, "two"); err == nil {
		t.Fatal("duplicate fire accepted")
	}
}

func TestCorruptLedgerHasRecoveryInstructions(t *testing.T) {
	path := scratchDB(t)
	if e := os.WriteFile(path, []byte("not a sqlite database"), 0600); e != nil {
		t.Fatal(e)
	}
	_, e := Open(context.Background(), path)
	if e == nil || !strings.Contains(e.Error(), path) || !strings.Contains(e.Error(), "move it aside") || !strings.Contains(e.Error(), "agentcron init") {
		t.Fatalf("%v", e)
	}
}
