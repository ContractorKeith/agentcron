package store

import (
	"context"
	"testing"
)

func TestFireUniquenessOnlyForScheduledTriggers(t *testing.T) {
	s, err := Open(context.Background(), scratchDB(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, trigger := range []string{"manual", "mcp", "schedule", "catchup"} {
		t.Run(trigger, func(t *testing.T) {
			insert := "INSERT INTO runs (id,job,trigger,scheduled_for,state) VALUES (?,'job',?,'2026-11-04','queued')"
			if _, err := s.db.Exec(insert, trigger+"-one", trigger); err != nil {
				t.Fatal(err)
			}
			_, err := s.db.Exec(insert, trigger+"-two", trigger)
			scheduled := trigger == "schedule" || trigger == "catchup"
			if scheduled && err == nil {
				t.Fatal("duplicate scheduled fire accepted")
			}
			if !scheduled && err != nil {
				t.Fatalf("independent %s run rejected: %v", trigger, err)
			}
		})
	}
}
