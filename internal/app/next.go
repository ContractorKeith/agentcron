package app

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ContractorKeith/agentcron/internal/clock"
	"github.com/ContractorKeith/agentcron/internal/schedule"
)

type Fire struct {
	Name string    `json:"name"`
	At   time.Time `json:"at"`
}

func NextFires(ctx context.Context, opts Options, c clock.Clock, expr, zone string, count int) ([]Fire, error) {
	if count < 1 || count > 1000 {
		return nil, &Error{Usage, fmt.Errorf("count: must be between 1 and 1000")}
	}
	s, err := openJobs(ctx, opts, c, false)
	if err != nil {
		return nil, err
	}
	defer s.db.Close()
	result := []Fire{}
	if expr != "" {
		parsed, err := schedule.Parse(expr)
		if err != nil {
			return nil, &Error{Usage, err}
		}
		loc := time.Local
		if zone != "" {
			loc, err = time.LoadLocation(zone)
			if err != nil {
				return nil, &Error{Usage, fmt.Errorf("timezone: %w", err)}
			}
		}
		for _, at := range fires(parsed.WithAnchor(s.now), s.now, loc, count) {
			result = append(result, Fire{At: at})
		}
		return result, nil
	}
	if zone != "" {
		return nil, &Error{Usage, fmt.Errorf("timezone requires schedule")}
	}
	for _, view := range s.views {
		if view.Error != "" || view.State.Paused || !view.State.Approved {
			continue
		}
		for _, at := range jobFires(*view.Job, view.State.AnchorAt, s.now, count) {
			result = append(result, Fire{Name: view.Name, At: at})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].At.Equal(result[j].At) {
			return result[i].Name < result[j].Name
		}
		return result[i].At.Before(result[j].At)
	})
	return result, nil
}
