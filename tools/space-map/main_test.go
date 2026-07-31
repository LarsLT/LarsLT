package main

import (
	"math"
	"testing"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/render"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/sources"
)

// vandenberg is the busiest pad in the feed and the one the plain formula gets
// wrong, so it carries most of the aiming cases worth pinning.
var vandenberg = geo.Point{Lon: -120.611, Lat: 34.632}

// TestAimRespectsTheRange is the whole point of the corridor table: a heading
// the range would never permit has to be pulled to one it would.
func TestAimRespectsTheRange(t *testing.T) {
	for _, tc := range []struct {
		name        string
		site        string
		pos         geo.Point
		inclination float64
		want        float64
	}{
		// Both roots are due east, which the Western Range does not fly, so this
		// is the case that has to clamp rather than pick the mirror.
		{"Vandenberg minimum energy", "Vandenberg SFB", vandenberg, 34.632, 140},
		{"Vandenberg Starlink shell", "Vandenberg SFB", vandenberg, 53, 140},
		{"Vandenberg polar", "Vandenberg SFB", vandenberg, 90, 180},
		{"Vandenberg sun-synchronous", "Vandenberg SFB", vandenberg, 97.8, 189.5},

		// Kourou flies sun-synchronous north over the Atlantic, which is the
		// mirror of what the formula hands back.
		{"Guiana sun-synchronous", "Guiana Space Centre", geo.Point{Lon: -52.77, Lat: 5.24}, 97.8, 352.2},
		{"Guiana geostationary transfer", "Guiana Space Centre", geo.Point{Lon: -52.77, Lat: 5.24}, 5.24, 90},

		{"Canaveral geostationary transfer", "Cape Canaveral SFS", geo.Point{Lon: -80.577, Lat: 28.56}, 28.56, 90},
		{"Canaveral GPS", "Cape Canaveral SFS", geo.Point{Lon: -80.577, Lat: 28.56}, 55, 40.7},

		// An unlisted site keeps whatever the formula said, which is how the
		// pads nobody has written a corridor for carry on working.
		{"Rocket Lab sun-synchronous", "Rocket Lab Launch Complex 1", geo.Point{Lon: 177.86, Lat: -39.26}, 97.8, 190.1},
	} {
		got := aim(sources.Launch{Site: tc.site, Position: tc.pos}, tc.inclination)
		if math.Abs(got-tc.want) > 0.1 {
			t.Errorf("%s: aim = %.1f, want %.1f", tc.name, got, tc.want)
		}
	}
}

// TestAscentOnlyOnTheDay stops the arc standing on the map for weeks, and stops
// the dot moving for a rocket that has not left the pad.
func TestAscentOnlyOnTheDay(t *testing.T) {
	now := time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC)
	pad := sources.Launch{Site: "Vandenberg SFB", Orbit: "LEO", Position: vandenberg}

	for _, tc := range []struct {
		name  string
		at    time.Time
		orbit string
		fly   bool
		arc   bool
		dot   bool
	}{
		{"three weeks out", now.Add(21 * 24 * time.Hour), "LEO", false, false, false},
		{"tomorrow", now.Add(20 * time.Hour), "LEO", false, false, false},
		{"later today", now.Add(8 * time.Hour), "LEO", false, true, false},
		{"earlier today, airborne", now.Add(-4 * time.Minute), "LEO", true, true, true},
		{"today but suborbital", now.Add(8 * time.Hour), "Sub", false, false, false},
	} {
		l := pad
		l.At, l.Orbit, l.Flying = tc.at, tc.orbit, tc.fly

		var sky render.Sky
		addAscent(&sky, l, now)

		if arc := sky.Ascent != nil; arc != tc.arc {
			t.Errorf("%s: arc drawn = %v, want %v", tc.name, arc, tc.arc)
			continue
		}
		if !tc.arc {
			continue
		}
		if dot := sky.Ascent.Ride != ""; dot != tc.dot {
			t.Errorf("%s: dot moving = %v, want %v", tc.name, dot, tc.dot)
		}
	}
}

// TestLaunchWhen pins the one rule the label has: it prints a clock only when
// the feed gave one, and a rocket already climbing does not get a time at all.
func TestLaunchWhen(t *testing.T) {
	at := time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		launch sources.Launch
		want   string
	}{
		{"minute precision", sources.Launch{At: at}, "01 Aug 02:00Z"},
		{"day precision", sources.Launch{At: at, Vague: true}, "01 Aug (TBD)"},
		{"airborne", sources.Launch{At: at, Flying: true}, "lifting off"},
		{"airborne beats vague", sources.Launch{At: at, Vague: true, Flying: true}, "lifting off"},
	} {
		if got := launchWhen(tc.launch); got != tc.want {
			t.Errorf("%s: launchWhen = %q, want %q", tc.name, got, tc.want)
		}
	}
}
