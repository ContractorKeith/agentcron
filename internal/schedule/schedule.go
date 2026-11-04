// Package schedule parses expressions without starting a scheduler or reading a clock.
package schedule

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

type Schedule struct {
	cron     cron.Schedule
	interval time.Duration
	at       time.Time
	oneShot  bool
	anchor   time.Time
}

func Parse(expr string) (Schedule, error) {
	if strings.HasPrefix(expr, "every ") {
		d, err := time.ParseDuration(strings.TrimPrefix(expr, "every "))
		if err != nil || d < time.Minute {
			return Schedule{}, fmt.Errorf("schedule: every duration must be at least 1m")
		}
		return Schedule{interval: d}, nil
	}
	if strings.HasPrefix(expr, "at ") {
		at, err := time.Parse(time.RFC3339, strings.TrimPrefix(expr, "at "))
		if err != nil {
			return Schedule{}, fmt.Errorf("schedule: %w", err)
		}
		return Schedule{at: at, oneShot: true}, nil
	}
	macros := map[string]string{"@hourly": "0 * * * *", "@daily": "0 0 * * *", "@weekly": "0 0 * * 0", "@monthly": "0 0 1 * *"}
	if expanded, ok := macros[expr]; ok {
		expr = expanded
	} else if strings.HasPrefix(expr, "@") {
		return Schedule{}, fmt.Errorf("schedule: unsupported macro %q", expr)
	}
	// ParseStandard also accepts TZ prefixes; the job timezone is the sole authority.
	if len(strings.Fields(expr)) != 5 {
		return Schedule{}, fmt.Errorf("schedule: expected five cron fields")
	}
	c, err := cron.ParseStandard(expr)
	if err != nil {
		return Schedule{}, fmt.Errorf("schedule: %w", err)
	}
	return Schedule{cron: c}, nil
}

// WithAnchor fixes interval alignment to the job's persisted first-seen time.
func (s Schedule) WithAnchor(anchor time.Time) Schedule { s.anchor = anchor; return s }

func (s Schedule) Next(after time.Time, loc *time.Location) (time.Time, bool) {
	if loc == nil {
		loc = time.Local
	}
	if s.oneShot {
		return s.at, s.at.After(after)
	}
	if s.interval > 0 {
		anchor := s.anchor
		if anchor.IsZero() {
			anchor = after
		}
		if after.Before(anchor) {
			return anchor, true
		}
		return anchor.Add((after.Sub(anchor)/s.interval + 1) * s.interval), true
	}
	for {
		next := s.cron.Next(after.In(loc))
		if next.IsZero() {
			return time.Time{}, false
		}
		if !secondOccurrence(next, loc) {
			return next, true
		}
		after = next
	}
}

// Compare offsets around the transition, then test whether this wall time occurred earlier.
func secondOccurrence(t time.Time, loc *time.Location) bool {
	local := t.In(loc)
	_, offset := local.Zone()
	for _, delta := range []time.Duration{-48 * time.Hour, -24 * time.Hour, -6 * time.Hour} {
		_, previous := t.Add(delta).In(loc).Zone()
		if previous <= offset {
			continue
		}
		earlier := t.Add(-time.Duration(previous-offset) * time.Second).In(loc)
		if earlier.Format("2006-01-02T15:04:05") == local.Format("2006-01-02T15:04:05") {
			return true
		}
	}
	return false
}
