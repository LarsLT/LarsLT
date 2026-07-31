package astro

import (
	"math"
	"testing"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// solarElevation is the textbook formula, kept here as an independent check on
// the night polygon: a point is lit exactly when the sun is above its horizon.
func solarElevation(p geo.Point, subsolar geo.Point) float64 {
	hourAngle := (p.Lon - subsolar.Lon) * deg
	sinAlt := math.Sin(p.Lat*deg)*math.Sin(subsolar.Lat*deg) +
		math.Cos(p.Lat*deg)*math.Cos(subsolar.Lat*deg)*math.Cos(hourAngle)
	return math.Asin(sinAlt) * rad
}

// inPolygon is ray casting in lon/lat space. The night polygon never crosses
// the antimeridian, so no wrapping is needed here.
func inPolygon(p geo.Point, poly []geo.Point) bool {
	inside := false
	for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
		a, b := poly[i], poly[j]
		if (a.Lat > p.Lat) == (b.Lat > p.Lat) {
			continue
		}
		x := (b.Lon-a.Lon)*(p.Lat-a.Lat)/(b.Lat-a.Lat) + a.Lon
		if p.Lon < x {
			inside = !inside
		}
	}
	return inside
}

func TestSubsolarPointKnownValues(t *testing.T) {
	cases := []struct {
		name    string
		when    time.Time
		wantLat float64
		tol     float64
	}{
		{"june solstice", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC), 23.44, 0.1},
		{"december solstice", time.Date(2026, 12, 21, 12, 0, 0, 0, time.UTC), -23.44, 0.1},
		{"march equinox", time.Date(2026, 3, 20, 14, 46, 0, 0, time.UTC), 0, 0.1},
		{"september equinox", time.Date(2026, 9, 23, 0, 5, 0, 0, time.UTC), 0, 0.1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := SubsolarPoint(c.when)
			if math.Abs(got.Lat-c.wantLat) > c.tol {
				t.Errorf("declination = %.3f, want %.2f (±%.2f)", got.Lat, c.wantLat, c.tol)
			}
		})
	}
}

// Two positions worked out from Meeus' apparent solar coordinates. The noon
// test below would wave a degree of sidereal-time error through; this will not.
func TestSubsolarLongitudeAgainstReference(t *testing.T) {
	for _, c := range []struct {
		when     time.Time
		lat, lon float64
	}{
		{time.Date(2026, 7, 30, 17, 9, 0, 0, time.UTC), 18.379, -75.634},
		{time.Date(2026, 6, 21, 8, 25, 0, 0, time.UTC), 23.438, 54.199},
	} {
		got := SubsolarPoint(c.when)
		if math.Abs(got.Lat-c.lat) > 0.005 {
			t.Errorf("%s: declination %.4f, want %.3f", c.when.Format(time.RFC3339), got.Lat, c.lat)
		}
		if off := geo.WrapLon(got.Lon - c.lon); math.Abs(off) > 0.005 {
			t.Errorf("%s: subsolar longitude %.4f, want %.3f", c.when.Format(time.RFC3339), got.Lon, c.lon)
		}
	}
}

// Everything the sun and the ground track do hangs off this one constant: how
// far the Earth had already turned under the stars at J2000.
func TestGreenwichAngleAtJ2000(t *testing.T) {
	got := GreenwichAngle(time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC))
	if math.Abs(got-280.460618) > 1e-6 {
		t.Errorf("GMST at J2000 = %.9f, want 280.460618", got)
	}
}

// At 12:00 UTC the sun stands over the Greenwich meridian, off only by the
// equation of time, which never exceeds about 16 minutes, or 4 degrees.
func TestSubsolarPointNoonLongitude(t *testing.T) {
	for day := 0; day < 365; day += 7 {
		when := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, day)
		lon := SubsolarPoint(when).Lon
		if math.Abs(lon) > 4.2 {
			t.Errorf("%s: subsolar longitude at noon UTC = %.2f, want within ±4.2", when.Format(time.DateOnly), lon)
		}
	}
}

// The sun crosses the whole map once a day and always westward, which is the
// premise of the sliding terminator animation.
func TestSubsolarPointDriftsWestward(t *testing.T) {
	start := time.Date(2026, 7, 30, 0, 0, 0, 0, time.UTC)
	prev := SubsolarPoint(start).Lon
	total := 0.0
	for h := 1; h <= 24; h++ {
		lon := SubsolarPoint(start.Add(time.Duration(h) * time.Hour)).Lon
		step := lon - prev
		if step > 180 {
			step -= 360
		}
		if step < -180 {
			step += 360
		}
		if step > 0 {
			t.Fatalf("hour %d: subsolar longitude moved east by %.2f", h, step)
		}
		total += step
		prev = lon
	}
	if math.Abs(total+360) > 1 {
		t.Errorf("24 h of drift = %.2f degrees, want -360 (±1)", total)
	}
}

// Shifting a longitude and the sun by the same amount leaves the terminator
// latitude untouched, which is what makes the translated copy exact.
func TestTerminatorSlidesExactlyAtFixedDeclination(t *testing.T) {
	const declination = 18.4
	for _, shift := range []float64{15, 90, 180, 270} {
		for lon := -180.0; lon <= 180; lon += 5 {
			want := TerminatorLatitude(lon, -75.0, declination)
			got := TerminatorLatitude(lon+shift, -75.0+shift, declination)
			if math.Abs(got-want) > 1e-9 {
				t.Fatalf("shift %.0f at lon %.0f: %.9f != %.9f", shift, lon, got, want)
			}
		}
	}
}

// A year of samples: dark inside the polygon, lit outside. Points within a
// degree of the boundary are the tracing step, not a disagreement.
func TestNightPolygonMatchesSolarElevation(t *testing.T) {
	const boundaryBand = 1.0

	start := time.Date(2026, 1, 1, 3, 17, 0, 0, time.UTC)
	for step := range 60 {
		when := start.Add(time.Duration(step) * 149 * time.Hour)
		subsolar := SubsolarPoint(when)
		night := NightPolygon(subsolar, 1.0)

		for lat := -85.0; lat <= 85; lat += 5 {
			for lon := -179.0; lon <= 179; lon += 7 {
				p := geo.Point{Lon: lon, Lat: lat}
				elevation := solarElevation(p, subsolar)
				if math.Abs(elevation) < boundaryBand {
					continue
				}
				dark := inPolygon(p, night)
				if dark != (elevation < 0) {
					t.Fatalf("%s: %.0fN %.0fE sun elevation %.2f, polygon says dark=%v",
						when.Format(time.RFC3339), lat, lon, elevation, dark)
				}
			}
		}
	}
}

// Getting this backwards would flood the wrong half of the map, so both
// hemispheres get checked.
func TestNightPolygonClosesOverTheDarkPole(t *testing.T) {
	cases := []struct {
		name      string
		when      time.Time
		darkPole  geo.Point
		lightPole geo.Point
	}{
		{
			"northern summer",
			time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC),
			geo.Point{Lon: 20, Lat: -88},
			geo.Point{Lon: 20, Lat: 88},
		},
		{
			"northern winter",
			time.Date(2026, 12, 21, 9, 0, 0, 0, time.UTC),
			geo.Point{Lon: 20, Lat: 88},
			geo.Point{Lon: 20, Lat: -88},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			night := NightPolygon(SubsolarPoint(c.when), 2.0)
			if !inPolygon(c.darkPole, night) {
				t.Errorf("polar night at %.0fN is not covered", c.darkPole.Lat)
			}
			if inPolygon(c.lightPole, night) {
				t.Errorf("polar day at %.0fN is covered", c.lightPole.Lat)
			}
		})
	}
}

// The traced edge has to span the full map, or the fill leaves a wedge of the
// world permanently lit.
func TestNightPolygonSpansTheWholeMap(t *testing.T) {
	night := NightPolygon(SubsolarPoint(time.Date(2026, 7, 30, 17, 9, 0, 0, time.UTC)), 2.0)
	minLon, maxLon := math.Inf(1), math.Inf(-1)
	for _, p := range night {
		minLon = math.Min(minLon, p.Lon)
		maxLon = math.Max(maxLon, p.Lon)
	}
	if minLon > -180 || maxLon < 180 {
		t.Errorf("polygon spans %.1f..%.1f, want -180..180", minLon, maxLon)
	}
}

// Every point traced has to be a place the sun sits exactly that far under the
// horizon, which is the one thing the aurora mask rests on.
func TestDarkPolygonFollowsTheDepression(t *testing.T) {
	for _, c := range []struct {
		name string
		when time.Time
		pole bool
	}{
		{"northern summer", time.Date(2026, time.July, 31, 9, 47, 0, 0, time.UTC), true},
		{"southern summer", time.Date(2026, time.January, 12, 3, 0, 0, 0, time.UTC), true},
		{"equinox", time.Date(2026, time.September, 22, 18, 0, 0, 0, time.UTC), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			subsolar := SubsolarPoint(c.when)
			ring := DarkPolygon(subsolar, 12, 2)
			if len(ring) == 0 {
				t.Fatal("traced nothing")
			}

			touchedPole := false
			for _, p := range ring {
				if math.Abs(p.Lat) == 90 {
					touchedPole = true
					continue
				}
				if got := SunElevation(p, subsolar); math.Abs(got+12) > 1e-6 {
					t.Fatalf("at %.0f,%.0f the sun is %.4f up, want 12 down", p.Lon, p.Lat, got)
				}
			}
			if touchedPole != c.pole {
				t.Errorf("closed along a pole: %v, want %v", touchedPole, c.pole)
			}
		})
	}
}

// Nautical twilight is a smaller region than plain night, and it has to sit
// inside it rather than wander off somewhere else.
func TestDarkerThanNightIsInsideNight(t *testing.T) {
	subsolar := SubsolarPoint(time.Date(2026, time.July, 31, 9, 47, 0, 0, time.UTC))
	for _, p := range DarkPolygon(subsolar, 12, 2) {
		if math.Abs(p.Lat) == 90 {
			continue
		}
		if SunElevation(p, subsolar) >= 0 {
			t.Fatalf("point %.0f,%.0f is in daylight", p.Lon, p.Lat)
		}
	}
}
