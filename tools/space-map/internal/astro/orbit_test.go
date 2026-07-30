package astro

import (
	"math"
	"testing"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// A real ISS element set, kept in its three pieces so a test can damage one
// field without disturbing the fixed columns around it.
const (
	issName  = "ISS (ZARYA)"
	issLine1 = "1 25544U 98067A   26211.15218593  .00008758  00000+0  16539-3 0  9990"
	issLine2 = "2 25544  51.6319  87.4712 0007078 352.8836   7.2051 15.49259390578410"
	issTLE   = issName + "\n" + issLine1 + "\n" + issLine2
)

// splice overwrites a fixed-width field without moving the columns around it.
func splice(line string, at int, field string) string {
	return line[:at] + field + line[at+len(field):]
}

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

	// Fixes from wheretheiss.at, all around fourteen hours after epoch. The
	// mid-latitude ones run near the limit: they are geodetic, this is not.
	for _, c := range []struct {
		unix             int64
		wantLat, wantLon float64
	}{
		{1785432926, 0.95260791060048, -126.8826394683},
		{1785433902, 45.151857971996, -79.444327820216},
		{1785443056, -45.192543762571, 137.03449491698},
	} {
		got := tle.SubPoint(time.Unix(c.unix, 0).UTC())
		offBy := math.Hypot(got.Lat-c.wantLat, geo.WrapLon(got.Lon-c.wantLon)*math.Cos(c.wantLat*deg)) * 111
		if offBy > 25 {
			t.Errorf("sub-point %.4fN %.4fE is %.1f km from the fix %.4fN %.4fE",
				got.Lat, got.Lon, offBy, c.wantLat, c.wantLon)
		}
	}
}

// Drag is what separates a fresh element set from a stale one, and the reason
// the epoch gets a staleness cap: a couple of hundred kilometres at a week.
func TestDragTermCarriesTheOrbitForward(t *testing.T) {
	tle := mustParse(t)
	if got := tle.MeanMotionDot * 86400 * 86400 / (2 * math.Pi); math.Abs(got-0.00008758) > 1e-12 {
		t.Errorf("mean motion rate %.11f rev per day squared, want 0.00008758", got)
	}

	bare := *tle
	bare.MeanMotionDot = 0
	drift := func(days float64) float64 {
		at := tle.Epoch.Add(time.Duration(days * 24 * float64(time.Hour)))
		a, b := tle.SubPoint(at), bare.SubPoint(at)
		return math.Hypot(a.Lat-b.Lat, geo.WrapLon(a.Lon-b.Lon)*math.Cos(a.Lat*deg)) * 111
	}
	if week := drift(7); week < 150 || week > 200 {
		t.Errorf("a week past epoch drag moves the sub-point %.0f km, want 150..200", week)
	}
	if ratio := drift(7) / drift(3.5); math.Abs(ratio-4) > 0.2 {
		t.Errorf("drift grew %.2f times over twice the span, want 4", ratio)
	}

	// Drag speeds the orbit up, so the sub-point has to run ahead of the
	// drag-free one along the track rather than trail it.
	at := tle.Epoch.Add(7 * 24 * time.Hour)
	a, b := tle.SubPoint(at), bare.SubPoint(at)
	ahead := bare.SubPoint(at.Add(10 * time.Second))
	along := (a.Lat-b.Lat)*(ahead.Lat-b.Lat) +
		geo.WrapLon(a.Lon-b.Lon)*geo.WrapLon(ahead.Lon-b.Lon)*math.Cos(b.Lat*deg)
	if !(along > 0) {
		t.Errorf("drag pushed the sub-point backwards along the track")
	}
}

// Nothing upstream promises well-formed input: the feed can time out into an
// HTML page or arrive clipped, and none of it may take the generator down.
func TestParseTLERejectsMalformedInput(t *testing.T) {
	for _, c := range []struct{ name, text string }{
		{"empty", ""},
		{"whitespace only", "  \n\t\n   \n"},
		{"name only", issName},
		{"line 1 alone", issName + "\n" + issLine1},
		{"line 2 alone", issName + "\n" + issLine2},
		{"line 1 a column short", issName + "\n" + issLine1[:63] + "\n" + issLine2},
		{"line 2 a column short", issName + "\n" + issLine1 + "\n" + issLine2[:62]},
		{"an HTML error page", "<html>\n<head><title>502 Bad Gateway</title></head>\n<body>502</body>\n</html>"},
		{"non-numeric epoch", issName + "\n" + splice(issLine1, 18, "xxxxxxxxxxxxxx") + "\n" + issLine2},
		{"non-numeric drag", issName + "\n" + splice(issLine1, 33, "xxxxxxxxxx") + "\n" + issLine2},
		{"non-numeric inclination", issName + "\n" + issLine1 + "\n" + splice(issLine2, 8, "xxxxxxxx")},
		{"non-numeric eccentricity", issName + "\n" + issLine1 + "\n" + splice(issLine2, 26, "xxxxxxx")},
		{"non-numeric mean motion", issName + "\n" + issLine1 + "\n" + splice(issLine2, 52, "xxxxxxxxxxx")},
		{"zero mean motion", issName + "\n" + issLine1 + "\n" + splice(issLine2, 52, "00.00000000")},
		{"negative mean motion", issName + "\n" + issLine1 + "\n" + splice(issLine2, 52, "-1.49259390")},
	} {
		t.Run(c.name, func(t *testing.T) {
			if tle, err := ParseTLE(c.text); err == nil {
				t.Fatalf("parsed into %+v, want an error", tle)
			}
		})
	}
}

// The shapes a real feed does deliver: CRLF line ends, no name line, and lines
// clipped after the last column this parser reads.
func TestParseTLEAcceptsRoughlyDeliveredInput(t *testing.T) {
	want := mustParse(t)
	for _, c := range []struct{ name, text string }{
		{"CRLF without a name", issLine1 + "\r\n" + issLine2 + "\r\n"},
		{"line 1 clipped past the fields it reads", issName + "\n" + issLine1[:66] + "\n" + issLine2},
		{"line 2 clipped to its last field", issName + "\n" + issLine1 + "\n" + issLine2[:63]},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseTLE(c.text)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if !got.Epoch.Equal(want.Epoch) || got.MeanMotion != want.MeanMotion ||
				got.MeanMotionDot != want.MeanMotionDot || got.Inclination != want.Inclination {
				t.Errorf("parsed %+v, want the orbit in %+v", got, want)
			}
		})
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
