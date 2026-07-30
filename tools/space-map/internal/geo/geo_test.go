package geo

import (
	"math"
	"testing"
)

// Both spellings of the seam have to fold to the same edge, or a track that
// ends on +180 and one that starts on -180 read as half a world apart.
func TestWrapLonAtTheSeam(t *testing.T) {
	for _, c := range []struct{ in, want float64 }{
		{180, -180},
		{-180, -180},
		{540, -180},
		{-540, -180},
		{179.9, 179.9},
		{180.1, -179.9},
		{-180.1, 179.9},
		{0, 0},
		{360, 0},
		{-450, -90},
	} {
		if got := WrapLon(c.in); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("WrapLon(%.1f) = %.4f, want %.1f", c.in, got, c.want)
		}
	}
}

// One crossing, one split: the run before the seam ends on the last point east
// of it and the next begins on the first point west, with nothing dropped.
func TestSplitAntimeridianSplitsOnceAtTheSeam(t *testing.T) {
	track := []Point{
		{Lon: 165, Lat: 10},
		{Lon: 175, Lat: 11},
		{Lon: -175, Lat: 12},
		{Lon: -165, Lat: 13},
	}
	runs := SplitAntimeridian(track)
	if len(runs) != 2 {
		t.Fatalf("got %d runs, want 2", len(runs))
	}
	if len(runs[0]) != 2 || len(runs[1]) != 2 {
		t.Fatalf("runs split %d/%d, want 2/2", len(runs[0]), len(runs[1]))
	}
	if runs[0][1].Lon != 175 || runs[1][0].Lon != -175 {
		t.Errorf("split between %.0f and %.0f, want between 175 and -175", runs[0][1].Lon, runs[1][0].Lon)
	}
	if runs[0][1].Lat != 11 || runs[1][0].Lat != 12 {
		t.Error("the split lost the latitudes that go with the seam points")
	}
}

// A track that never reaches the seam comes back whole, and it is the wrapped
// longitudes that decide that, not the ones handed in.
func TestSplitAntimeridianLeavesAWholeTrackAlone(t *testing.T) {
	for _, c := range []struct {
		name  string
		track []Point
	}{
		{"inside the map", []Point{{Lon: -30}, {Lon: 0}, {Lon: 30}, {Lon: 60}}},
		{"a lap unwrapped", []Point{{Lon: 370}, {Lon: 380}, {Lon: 390}}},
	} {
		t.Run(c.name, func(t *testing.T) {
			runs := SplitAntimeridian(c.track)
			if len(runs) != 1 {
				t.Fatalf("got %d runs, want 1", len(runs))
			}
			if len(runs[0]) != len(c.track) {
				t.Errorf("run has %d points, want %d", len(runs[0]), len(c.track))
			}
			for _, p := range runs[0] {
				if p.Lon < -180 || p.Lon > 180 {
					t.Errorf("longitude %.0f came back unwrapped", p.Lon)
				}
			}
		})
	}
	if runs := SplitAntimeridian(nil); len(runs) != 0 {
		t.Errorf("an empty track produced %d runs", len(runs))
	}
}
