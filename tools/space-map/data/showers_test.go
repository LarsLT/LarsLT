package data

import (
	"testing"
	"time"
)

func showerNamed(t *testing.T, name string) Shower {
	t.Helper()
	for _, s := range Showers {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no shower called %q", name)
	return Shower{}
}

// The Quadrantids open in December and peak in January, so the countdown has to
// walk over the year boundary instead of jumping back most of a year.
func TestPeakInAcrossNewYear(t *testing.T) {
	quadrantids := showerNamed(t, "Quadrantids")
	for _, c := range []struct {
		when time.Time
		want int
	}{
		{time.Date(2025, time.December, 28, 22, 0, 0, 0, time.UTC), 6},
		{time.Date(2025, time.December, 31, 22, 0, 0, 0, time.UTC), 3},
		{time.Date(2026, time.January, 1, 3, 0, 0, 0, time.UTC), 2},
		{time.Date(2026, time.January, 3, 3, 0, 0, 0, time.UTC), 0},
		{time.Date(2026, time.January, 6, 3, 0, 0, 0, time.UTC), -3},
	} {
		if got := quadrantids.PeakIn(c.when); got != c.want {
			t.Errorf("%s: PeakIn = %d, want %d", c.when.Format(time.DateOnly), got, c.want)
		}
	}
}

// A shower that is not running on its own peak day would have the map announce
// a peak for something it never draws.
func TestEveryShowerIsActiveOnItsOwnPeak(t *testing.T) {
	for _, s := range Showers {
		peak := time.Date(2026, s.Peak.Month, s.Peak.Day, 12, 0, 0, 0, time.UTC)
		if !s.ActiveOn(peak) {
			t.Errorf("%s peaks on %s but is not active that day", s.Name, peak.Format(time.DateOnly))
		}
		if got := s.PeakIn(peak); got != 0 {
			t.Errorf("%s: PeakIn on its own peak = %d, want 0", s.Name, got)
		}
		if best, ok := StrongestShower(peak); !ok || best.ZHR < s.ZHR {
			t.Errorf("%s peaks at ZHR %d but the strongest shower that day is %v", s.Name, s.ZHR, best.Name)
		}
	}
}
