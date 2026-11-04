package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFailedCallbackReleasesWriter(t *testing.T) {
	ctx := context.Background()
	s, err := Open(ctx, scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.db.SetMaxOpenConns(1)
	stop := errors.New("callback failed")
	err = s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		if _, err := tx.Exec(ctx, "INSERT INTO job_state (name,updated_at) VALUES ('rollback','now')"); err != nil {
			return err
		}
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatal(err)
	}
	if err := s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
		_, err := tx.Exec(ctx, "INSERT INTO job_state (name,updated_at) VALUES ('next','now')")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := s.db.QueryRow("SELECT name FROM job_state").Scan(&name); err != nil || name != "next" {
		t.Fatalf("row %q: %v", name, err)
	}
}

// database/sql must release a cancelled transaction even while its callback unwinds.
func TestCancelledTransactionReleasesPooledConnection(t *testing.T) {
	s, err := Open(context.Background(), scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	s.db.SetMaxOpenConns(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ready, release := make(chan struct{}), make(chan struct{})
	first, second := make(chan error, 1), make(chan error, 1)
	stop := errors.New("callback failed after cancellation")
	go func() {
		first <- s.WithImmediateTx(ctx, func(tx *ImmediateTx) error {
			if _, err := tx.Exec(ctx, "INSERT INTO job_state (name,updated_at) VALUES ('cancelled','now')"); err != nil {
				return err
			}
			close(ready)
			<-release
			return stop
		})
	}()
	select {
	case <-ready:
	case err := <-first:
		close(release)
		t.Fatal(err)
	}
	cancel()
	go func() {
		second <- s.WithImmediateTx(context.Background(), func(tx *ImmediateTx) error {
			_, err := tx.Exec(context.Background(), "INSERT INTO job_state (name,updated_at) VALUES ('next','now')")
			return err
		})
	}()
	var writerErr error
	released := false
	select {
	case writerErr = <-second:
		released = true
	case <-time.After(time.Second):
	}
	close(release)
	if err := <-first; !errors.Is(err, stop) {
		t.Error(err)
	}
	if !released {
		writerErr = <-second
	}
	if writerErr != nil {
		t.Error(writerErr)
	}
	if !released {
		t.Fatal("cancelled transaction retained the pooled connection until the callback returned")
	}
	var name string
	if err := s.db.QueryRow("SELECT name FROM job_state").Scan(&name); err != nil || name != "next" {
		t.Fatalf("row %q: %v", name, err)
	}
}
