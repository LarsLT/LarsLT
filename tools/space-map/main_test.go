package main

import (
	"testing"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/sources"
)

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
