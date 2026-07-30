package astro

import (
	"math"
	"testing"
	"time"
)

// A real ISS element set, with a position fix taken from wheretheiss.at almost
// fourteen hours after its epoch.
const issTLE = `ISS (ZARYA)
1 25544U 98067A   26211.15218593  .00008758  00000+0  16539-3 0  9990
2 25544  51.6319  87.4712 0007078 352.8836   7.2051 15.49259390578410`

func mustParse(t *testing.T) *TLE {
	t.Helper()
	tle, err := ParseTLE(issTLE)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return tle
}

func TestParseTLEReadsTheElements(t *testing.T) {
	tle := mustParse(t)

	wantEpoch := time.Date(2026, 7, 30, 3, 39, 8, 0, time.UTC)
	if diff := tle.Epoch.Sub(wantEpoch); diff.Abs() > time.Second {
		t.Errorf("epoch %s, want %s", tle.Epoch.Format(time.RFC3339), wantEpoch.Format(time.RFC3339))
	}
	if got := tle.Inclination * rad; math.Abs(got-51.6319) > 1e-4 {
		t.Errorf("inclination %.4f, want 51.6319", got)
	}
	if math.Abs(tle.Eccentricity-0.0007078) > 1e-9 {
		t.Errorf("eccentricity %.7f, want 0.0007078", tle.Eccentricity)
	}
	if got := tle.Period(); (got - 92*time.Minute - 55*time.Second).Abs() > 5*time.Second {
		t.Errorf("period %s, want about 92m55s", got)
	}
}

// The whole reason there is no SGP4 dependency here. If this drifts, the
// simplification stopped being good enough and the claim needs revisiting.
func TestSubPointMatchesARealFix(t *testing.T) {
	tle := mustParse(t)

	at := time.Unix(1785432926, 0).UTC()
	const wantLat, wantLon = 0.95260791060048, -126.8826394683

	got := tle.SubPoint(at)
	offBy := math.Hypot(got.Lat-wantLat, (got.Lon-wantLon)*math.Cos(wantLat*deg)) * 111
	if offBy > 25 {
		t.Errorf("sub-point %.4fN %.4fE is %.1f km from the fix %.4fN %.4fE",
			got.Lat, got.Lon, offBy, wantLat, wantLon)
	}
}

// The track has to stay inside the orbit's own inclination, and get all the way
// round the world within one period.
func TestGroundTrackStaysWithinInclination(t *testing.T) {
	tle := mustParse(t)
	now := time.Unix(1785432926, 0).UTC()

	points, times := tle.GroundTrack(now, tle.Period(), 30*time.Second)
	if len(points) != len(times) {
		t.Fatalf("%d points but %d times", len(points), len(times))
	}
	if len(points) < 300 {
		t.Fatalf("only %d samples over two orbits", len(points))
	}

	limit := tle.Inclination * rad
	reachedNorth, reachedSouth := false, false
	for _, p := range points {
		if math.Abs(p.Lat) > limit+0.5 {
			t.Fatalf("track reached %.2fN, beyond the %.2f degree inclination", p.Lat, limit)
		}
		if p.Lat > limit-1 {
			reachedNorth = true
		}
		if p.Lat < -limit+1 {
			reachedSouth = true
		}
	}
	if !reachedNorth || !reachedSouth {
		t.Error("two orbits should touch both turning latitudes")
	}
}

// Epochs are written with a two-digit year, and the rollover is at 57.
func TestParseEpochHandlesTheCentury(t *testing.T) {
	for _, c := range []struct {
		field string
		year  int
	}{
		{"26211.15218593", 2026},
		{"57001.00000000", 1957},
		{"99001.00000000", 1999},
		{"00001.00000000", 2000},
	} {
		got, err := parseEpoch(c.field)
		if err != nil {
			t.Fatalf("%s: %v", c.field, err)
		}
		if got.Year() != c.year {
			t.Errorf("%s parsed as %d, want %d", c.field, got.Year(), c.year)
		}
	}
}
