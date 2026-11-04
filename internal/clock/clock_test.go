package clock

import (
	"testing"
	"time"
)

func TestFakeAdvances(t *testing.T) {
	start := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
	c := NewFake(start)
	first, second := c.After(time.Second), c.After(2*time.Second)
	c.Advance(time.Second)
	if got := <-first; !got.Equal(start.Add(time.Second)) {
		t.Fatal(got)
	}
	select {
	case <-second:
		t.Fatal("timer fired early")
	default:
	}
	c.Advance(3 * time.Second)
	if got := <-second; !got.Equal(start.Add(2 * time.Second)) {
		t.Fatal(got)
	}
	if !c.Now().Equal(start.Add(4 * time.Second)) {
		t.Fatal(c.Now())
	}
	if got := <-c.After(0); !got.Equal(c.Now()) {
		t.Fatal(got)
	}
	select {
	case <-first:
		t.Fatal("timer fired twice")
	default:
	}
}
