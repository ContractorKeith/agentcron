package app

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
)

func TestConcurrentAdd(t *testing.T) {
	opts := isolated(t)
	ctx := context.Background()
	if err := Init(ctx, opts); err != nil {
		t.Fatal(err)
	}
	c := clock.NewFake(time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC))
	start := make(chan struct{})
	results := make(chan string, 24)
	var wg sync.WaitGroup
	for i := range 24 {
		wg.Go(func() {
			<-start
			j := NewJob("race")
			j.Schedule = "@daily"
			j.Runner = "claude"
			j.Prompt = fmt.Sprintf("winner-%d", i)
			_, err := AddJob(ctx, opts, c, j, false)
			if err == nil {
				results <- j.Prompt
			} else if Code(err) != Usage {
				t.Errorf("add: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()
	close(results)
	winners := []string{}
	for prompt := range results {
		winners = append(winners, prompt)
	}
	if len(winners) != 1 {
		t.Fatalf("successful adds=%d, want 1", len(winners))
	}
	detail, err := ShowJob(ctx, opts, c, "race")
	if err != nil || detail.Job.Prompt != winners[0] {
		t.Fatalf("disk=%q winner=%q err=%v", detail.Job.Prompt, winners[0], err)
	}
}
