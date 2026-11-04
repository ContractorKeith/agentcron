package schedule

import (
	"testing"
	"time"
)

func instant(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}
func TestNext(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ expr, after, want string }{
		{"30 2 * * *", "2026-03-07T12:00:00Z", "2026-03-09T06:30:00Z"},
		{"30 1 * * *", "2026-11-01T04:00:00Z", "2026-11-01T05:30:00Z"},
		{"30 1 * * *", "2026-11-01T05:30:00Z", "2026-11-02T06:30:00Z"},
		{"30 1 * * *", "2026-11-01T06:15:00Z", "2026-11-02T06:30:00Z"},
		{"0 0 29 2 *", "2026-01-01T00:00:00Z", "2028-02-29T05:00:00Z"},
		{"@hourly", "2026-01-01T00:01:00Z", "2026-01-01T01:00:00Z"},
		{"@daily", "2026-03-08T05:01:00Z", "2026-03-09T04:00:00Z"},
		{"@weekly", "2026-03-09T00:00:00Z", "2026-03-15T04:00:00Z"},
		{"@monthly", "2026-03-09T00:00:00Z", "2026-04-01T04:00:00Z"},
		{"at 0001-01-01T00:00:00Z", "0000-12-31T23:59:00Z", "0001-01-01T00:00:00Z"},
		{"at 2026-11-05T09:00:00-05:00", "2026-11-04T00:00:00Z", "2026-11-05T14:00:00Z"},
	} {
		t.Run(tc.expr+tc.after, func(t *testing.T) {
			s, err := Parse(tc.expr)
			if err != nil {
				t.Fatal(err)
			}
			got, ok := s.Next(instant(tc.after), loc)
			if !ok || !got.Equal(instant(tc.want)) {
				t.Fatalf("got %s %t, want %s", got, ok, tc.want)
			}
		})
	}
}
func TestParseInvalid(t *testing.T) {
	for _, expr := range []string{"every 0s", "every 30s", "every -1h", "every nonsense", "@yearly", "* * * *", "60 * * * *", "at tomorrow", "CRON_TZ=UTC 0 1 * * *"} {
		if _, err := Parse(expr); err == nil {
			t.Errorf("accepted %q", expr)
		}
	}
}
func TestIntervalAnchorAndPastAt(t *testing.T) {
	s, _ := Parse("every 2h")
	s = s.WithAnchor(instant("2026-11-04T00:13:00Z"))
	got, ok := s.Next(instant("2026-11-04T03:00:00Z"), time.UTC)
	if !ok || !got.Equal(instant("2026-11-04T04:13:00Z")) {
		t.Fatal(got, ok)
	}
	got, ok = s.Next(instant("2026-11-03T23:00:00Z"), time.UTC)
	if !ok || !got.Equal(instant("2026-11-04T00:13:00Z")) {
		t.Fatal(got, ok)
	}
	at, _ := Parse("at 2026-11-04T00:00:00Z")
	for _, after := range []string{"2026-11-04T00:00:00Z", "2026-11-05T00:00:00Z"} {
		if _, ok := at.Next(instant(after), time.UTC); ok {
			t.Fatal("past at fires")
		}
	}
}
