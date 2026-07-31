package astro

import (
	"math"
	"testing"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// angularDistance between two points on a sphere, in degrees. The dot product
// needs clamping: rounding tips it past 1 for close points and Acos gives NaN.
func angularDistance(a, b geo.Point) float64 {
	dot := math.Sin(a.Lat*deg)*math.Sin(b.Lat*deg) +
		math.Cos(a.Lat*deg)*math.Cos(b.Lat*deg)*math.Cos((a.Lon-b.Lon)*deg)
	return math.Acos(math.Min(1, math.Max(-1, dot))) * rad
}

// Every traced point has to sit exactly on its small circle, which is the one
// thing the whole oval rests on.
func TestSmallCircleKeepsItsRadius(t *testing.T) {
	for _, magneticLat := range []float64{48, 58, 66.5, 73.5, 80} {
		wantRadius := 90 - magneticLat
		for lon := -180.0; lon <= 180; lon += 3 {
			p := geo.Point{Lon: lon, Lat: smallCircleLat(geomagneticNorth, magneticLat, lon)}
			got := angularDistance(p, geomagneticNorth)
			if math.IsNaN(got) || math.Abs(got-wantRadius) > 1e-6 {
				t.Fatalf("cgm %.1f at lon %.0f: radius %.6f, want %.6f", magneticLat, lon, got, wantRadius)
			}
		}
	}
}

// The southern oval is the northern one turned inside out, so it must ring the
// south geomagnetic pole at the same radius.
func TestSouthernOvalRingsTheSouthernPole(t *testing.T) {
	south := geo.Point{Lat: -geomagneticNorth.Lat, Lon: geo.WrapLon(geomagneticNorth.Lon + 180)}
	for lon := -180.0; lon <= 180; lon += 3 {
		p := geo.Point{Lon: lon, Lat: GeographicLatAt(66.5, lon, false)}
		if got := angularDistance(p, south); math.IsNaN(got) || math.Abs(got-23.5) > 1e-6 {
			t.Fatalf("lon %.0f: radius %.6f, want 23.5", lon, got)
		}
	}
	if lat := GeographicLatAt(66.5, 0, false); math.IsNaN(lat) || lat > 0 {
		t.Errorf("southern boundary at lon 0 came out at %.1fN", lat)
	}
}

// A storm has to push the glow towards the equator, never away from it.
func TestHigherKpReachesFurtherSouth(t *testing.T) {
	prev := 91.0
	for kp := 0.0; kp <= 9; kp++ {
		lat := GeographicLatAt(OvalBoundary(kp), 5, true)
		if math.IsNaN(lat) || lat >= prev {
			t.Fatalf("Kp %.0f reaches %.1fN, no further south than Kp %.0f at %.1fN", kp, lat, kp-1, prev)
		}
		prev = lat
	}
	// A severe storm is what puts the aurora over the Netherlands.
	if lat := GeographicLatAt(VisibleFrom(8), 5, true); math.IsNaN(lat) || lat > 53 {
		t.Errorf("Kp 8 is visible only down to %.1fN, expected it to reach the Netherlands", lat)
	}
}

// The band drawn is the ground people can see the glow from, so its equatorward
// edge has to be the horizon line and not the oval's own edge.
func TestOvalIsDrawnAsTheVisibleFootprint(t *testing.T) {
	const kp = 6
	ring := Oval(kp, true, 2)
	for _, p := range ring[:len(ring)/2] {
		want := GeographicLatAt(VisibleFrom(kp), p.Lon, true)
		if math.Abs(p.Lat-want) > 1e-9 {
			t.Fatalf("at lon %.0f the edge is %.2fN, want the %.2fN horizon line", p.Lon, p.Lat, want)
		}
		if overhead := GeographicLatAt(OvalBoundary(kp), p.Lon, true); p.Lat >= overhead {
			t.Fatalf("at lon %.0f the edge is %.2fN, not south of the oval's own %.2fN", p.Lon, p.Lat, overhead)
		}
	}
}

// smallCircleLat is single-rooted only below the pole's own latitude and Oval
// never checks that, so the constants feeding it get pinned here instead.
func TestOvalStaysInTheSingleRootRegion(t *testing.T) {
	if poleward := OvalBoundary(0) + ovalWidth; poleward >= geomagneticNorth.Lat {
		t.Errorf("the quiet oval reaches cgm %.1f, past the pole's own %.1f", poleward, geomagneticNorth.Lat)
	}
}

// Past that latitude a meridian can miss the circle altogether. Those answers
// park at the pole; what none of them may do is land off the circle.
func TestSmallCircleOutsideTheSingleRootRegion(t *testing.T) {
	const magneticLat = 82.0
	missed := 0
	for lon := -180.0; lon <= 180; lon++ {
		lat := smallCircleLat(geomagneticNorth, magneticLat, lon)
		if lat == 90 {
			missed++
			continue
		}
		got := angularDistance(geo.Point{Lon: lon, Lat: lat}, geomagneticNorth)
		if math.IsNaN(got) || math.Abs(got-(90-magneticLat)) > 1e-6 {
			t.Fatalf("lon %.0f: cgm %.0f came out at %.4fN, %.4f degrees off the pole", lon, magneticLat, lat, got)
		}
	}
	if missed == 0 {
		t.Fatal("a circle this small should leave meridians it never crosses")
	}
}

// The ring is a band between two edges that each span the map once, so both
// halves have to run the full width and never double back.
func TestOvalRingSpansTheMapTwice(t *testing.T) {
	for _, c := range []struct {
		name  string
		north bool
	}{{"north", true}, {"south", false}} {
		t.Run(c.name, func(t *testing.T) {
			ring := Oval(3, c.north, 2)
			if len(ring)%2 != 0 {
				t.Fatalf("ring has %d points, want an even split between the two edges", len(ring))
			}
			half := len(ring) / 2
			if ring[0].Lon != -180 || ring[half-1].Lon < 180 {
				t.Errorf("equatorward edge runs %.0f..%.0f, want -180..180", ring[0].Lon, ring[half-1].Lon)
			}
			if ring[half].Lon != 180 || ring[len(ring)-1].Lon > -180 {
				t.Errorf("poleward edge runs %.0f..%.0f, want 180..-180", ring[half].Lon, ring[len(ring)-1].Lon)
			}

			// Poleward means north of the other edge up north and south of it
			// down south, and a NaN is poleward of nothing.
			for i := range half {
				gap := ring[len(ring)-1-i].Lat - ring[i].Lat
				if !c.north {
					gap = -gap
				}
				if !(gap > 0) {
					t.Fatalf("at lon %.0f the poleward edge sits %.2f degrees from the equatorward one", ring[i].Lon, gap)
				}
			}
		})
	}
}
