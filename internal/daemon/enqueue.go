package daemon

import (
	"context"
	"time"

	"github.com/ContractorKeith/agentcron/internal/jobs"
	"github.com/ContractorKeith/agentcron/internal/schedule"
	"github.com/ContractorKeith/agentcron/internal/store"
)

func (e *Engine) Enqueue(ctx context.Context, defs map[string]jobs.Job, states map[string]store.JobState) error {
	now := e.Clock.Now().UTC().Truncate(time.Second)
	for name, j := range defs {
		st := states[name]
		if !st.Approved || st.Paused || st.Orphaned || st.NextFire == nil {
			continue
		}
		loc := time.Local
		if j.Timezone != "" {
			loc, _ = time.LoadLocation(j.Timezone)
		}
		s, _ := schedule.Parse(j.Schedule)
		s = s.WithAnchor(st.AnchorAt)
		fire := *st.NextFire
		var missed *time.Time
		var onTime []time.Time
		emit := func(at time.Time, trigger string) error {
			r, err := store.NewRun(name, trigger, at)
			if err != nil {
				return err
			}
			if err = e.DB.EnqueueFire(ctx, r); err != nil {
				return err
			}
			st.Fired = true
			if inserted, err := e.DB.GetRun(ctx, r.ID); err == nil {
				e.log(inserted)
			}
			return nil
		}
		for !fire.After(now) {
			if fire.Equal(now) {
				onTime = append(onTime, fire)
			} else if j.Catchup == "one" {
				t := fire
				missed = &t
			} else if fire.After(now.Add(-30 * time.Second)) {
				onTime = append(onTime, fire)
			}
			next, ok := s.Next(fire, loc)
			if !ok {
				st.NextFire = nil
				break
			}
			fire = next
			st.NextFire = &fire
		}
		if missed != nil {
			if err := emit(*missed, "catchup"); err != nil {
				return err
			}
		}
		for _, at := range onTime {
			if err := emit(at, "schedule"); err != nil {
				return err
			}
		}
		st.UpdatedAt = now
		if err := e.DB.AdvanceCursor(ctx, st); err != nil {
			return err
		}
	}
	return nil
}
