package astro

import (
	"math"
	"testing"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// angularDistance between two points on a sphere, in degrees.
func angularDistance(a, b geo.Point) float64 {
	return math.Acos(math.Sin(a.Lat*deg)*math.Sin(b.Lat*deg)+
		math.Cos(a.Lat*deg)*math.Cos(b.Lat*deg)*math.Cos((a.Lon-b.Lon)*deg)) * rad
}

// Every traced point has to sit exactly on its small circle, which is the one
// thing the whole oval rests on.
func TestSmallCircleKeepsItsRadius(t *testing.T) {
	for _, magneticLat := range []float64{48, 58, 66.5, 73.5, 80} {
		wantRadius := 90 - magneticLat
		for lon := -180.0; lon <= 180; lon += 3 {
			p := geo.Point{Lon: lon, Lat: smallCircleLat(geomagneticNorth, magneticLat, lon)}
			got := angularDistance(p, geomagneticNorth)
			if math.Abs(got-wantRadius) > 1e-6 {
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
		if got := angularDistance(p, south); math.Abs(got-23.5) > 1e-6 {
			t.Fatalf("lon %.0f: radius %.6f, want 23.5", lon, got)
		}
	}
	if lat := GeographicLatAt(66.5, 0, false); lat > 0 {
		t.Errorf("southern boundary at lon 0 came out at %.1fN", lat)
	}
}

// A storm has to push the glow towards the equator, never away from it.
func TestHigherKpReachesFurtherSouth(t *testing.T) {
	prev := 91.0
	for kp := 0.0; kp <= 9; kp++ {
		lat := GeographicLatAt(OvalBoundary(kp), 5, true)
		if lat >= prev {
			t.Fatalf("Kp %.0f reaches %.1fN, no further south than Kp %.0f at %.1fN", kp, lat, kp-1, prev)
		}
		prev = lat
	}
	// A severe storm is what puts the aurora over the Netherlands.
	if lat := GeographicLatAt(VisibleFrom(8), 5, true); lat > 53 {
		t.Errorf("Kp 8 is visible only down to %.1fN, expected it to reach the Netherlands", lat)
	}
}

// The ring is a band between two edges that each span the map once, so both
// halves have to run the full width and never double back.
func TestOvalRingSpansTheMapTwice(t *testing.T) {
	ring := Oval(3, true, 2)
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
	for i := range half {
		if ring[i].Lat >= ring[len(ring)-1-i].Lat {
			t.Fatalf("at lon %.0f the poleward edge is not poleward", ring[i].Lon)
		}
	}
}
