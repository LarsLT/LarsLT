package astro

import (
	"math"
	"testing"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// TestLaunchAzimuth checks the pads where the answer is not in dispute: due
// east for a minimum-energy flight, northeast for GPS, south for sun-synchronous.
func TestLaunchAzimuth(t *testing.T) {
	for _, tc := range []struct {
		name        string
		padLat      float64
		inclination float64
		want        float64
	}{
		{"Cape Canaveral GTO", 28.46, 28.46, 90.0},
		{"Guiana Centre GTO", 5.26, 5.26, 90.0},
		{"Cape Canaveral MEO", 28.46, 55, 40.7},
		{"Vandenberg SSO", 34.632, 97.8, 189.5},
		{"Rocket Lab NZ SSO", -39.26, 97.8, 190.1},
	} {
		got := LaunchAzimuth(tc.padLat, tc.inclination)
		if math.Abs(got-tc.want) > 0.1 {
			t.Errorf("%s: azimuth = %.1f, want %.1f", tc.name, got, tc.want)
		}
	}
}

// TestLaunchAzimuthClampsTheUnreachable covers an inclination no direct ascent
// from that latitude reaches. Due east is the closest it gets, and asin NaNs.
func TestLaunchAzimuthClampsTheUnreachable(t *testing.T) {
	for _, tc := range []struct {
		name        string
		padLat      float64
		inclination float64
		want        float64
	}{
		{"MEO from Andoya", 69.11, 55, 90},
		{"retrograde from Andoya", 69.11, 125, 270},
	} {
		got := LaunchAzimuth(tc.padLat, tc.inclination)
		if math.IsNaN(got) {
			t.Fatalf("%s: azimuth is NaN", tc.name)
		}
		if math.Abs(got-tc.want) > 0.1 {
			t.Errorf("%s: azimuth = %.1f, want %.1f", tc.name, got, tc.want)
		}
	}
}

// TestLaunchAzimuthIsAlwaysACompassBearing guards the normalisation: every
// answer has to be a bearing you could read off a compass.
func TestLaunchAzimuthIsAlwaysACompassBearing(t *testing.T) {
	for lat := -80.0; lat <= 80; lat += 10 {
		for inc := 0.0; inc <= 180; inc += 15 {
			az := LaunchAzimuth(lat, inc)
			if math.IsNaN(az) || az < 0 || az >= 360 {
				t.Fatalf("pad %.0f inclination %.0f: azimuth = %v", lat, inc, az)
			}
		}
	}
}

// TestMirrorAzimuthIsTheOtherRoot pins that the mirror reaches the same
// inclination and that mirroring twice comes back where it started.
func TestMirrorAzimuthIsTheOtherRoot(t *testing.T) {
	for _, az := range []float64{0, 40.7, 90, 189.5, 270, 352.2} {
		mirror := MirrorAzimuth(az)
		if mirror < 0 || mirror >= 360 {
			t.Errorf("mirror of %.1f is %.1f, off the compass", az, mirror)
		}
		if back := MirrorAzimuth(mirror); math.Abs(back-az) > 1e-9 {
			t.Errorf("mirroring %.1f twice gave %.1f", az, back)
		}
	}
}

// TestCorridorContains covers a plain span and one that wraps through north,
// which is how an easterly range like Kourou has to be written.
func TestCorridorContains(t *testing.T) {
	west := Corridor{From: 158, To: 201}
	kourou := Corridor{From: 349, To: 93}

	for _, tc := range []struct {
		name string
		c    Corridor
		az   float64
		want bool
	}{
		{"south is inside the western range", west, 189.5, true},
		{"due east is not", west, 90, false},
		{"due north is not", west, 0, false},
		{"the near edge counts", west, 158, true},
		{"the far edge counts", west, 201, true},
		{"east is inside a wrapping range", kourou, 90, true},
		{"just north of due north is too", kourou, 5, true},
		{"north-northwest is inside", kourou, 352.2, true},
		{"south is outside", kourou, 187.8, false},
	} {
		if got := tc.c.Contains(tc.az); got != tc.want {
			t.Errorf("%s: Contains(%.1f) = %v, want %v", tc.name, tc.az, got, tc.want)
		}
	}
}

// TestCorridorNearestClampsToAnEdge covers the case no root fits, where the arc
// has to be pulled to the closest heading the range would actually allow.
func TestCorridorNearestClampsToAnEdge(t *testing.T) {
	west := Corridor{From: 158, To: 201}

	if got := west.Nearest(189.5); got != 189.5 {
		t.Errorf("a bearing already inside moved to %.1f", got)
	}
	if got := west.Nearest(90); got != 158 {
		t.Errorf("due east clamped to %.1f, want the near edge 158", got)
	}
	if got := west.Nearest(250); got != 201 {
		t.Errorf("west-southwest clamped to %.1f, want the far edge 201", got)
	}
	// 260 is 102 from the near edge the short way round and 59 from the far one.
	if got := west.Nearest(260); got != 201 {
		t.Errorf("clamped the long way round to %.1f, want 201", got)
	}
}

// TestGreatCircleStaysOnTheSphere walks an arc off every pad in the feed and
// checks each sample is a real coordinate the right distance along.
func TestGreatCircleStaysOnTheSphere(t *testing.T) {
	from := geo.Point{Lon: -80.6, Lat: 28.46}
	arc := GreatCircle(from, 90, 25, 1)

	if len(arc) != 26 {
		t.Fatalf("sampled %d points, want 26", len(arc))
	}
	if angularDistance(arc[0], from) > 1e-9 {
		t.Errorf("arc starts at %v, want %v", arc[0], from)
	}
	for i, p := range arc {
		if math.IsNaN(p.Lat) || math.IsNaN(p.Lon) {
			t.Fatalf("point %d is %v", i, p)
		}
		if p.Lat < -90 || p.Lat > 90 || p.Lon < -180 || p.Lon > 180 {
			t.Errorf("point %d at %v is off the sphere", i, p)
		}
		if d := angularDistance(from, p); math.Abs(d-float64(i)) > 1e-6 {
			t.Errorf("point %d sits %.4f degrees along, want %d", i, d, i)
		}
	}
}

// TestGreatCircleDueEastFollowsTheEquator is the one case with an answer that
// can be written down: due east off the equator never leaves it.
func TestGreatCircleDueEastFollowsTheEquator(t *testing.T) {
	arc := GreatCircle(geo.Point{Lon: 0, Lat: 0}, 90, 30, 5)
	for i, p := range arc {
		if math.Abs(p.Lat) > 1e-9 {
			t.Errorf("point %d drifted to %.6fN", i, p.Lat)
		}
		if want := float64(i * 5); math.Abs(p.Lon-want) > 1e-9 {
			t.Errorf("point %d is at %.6fE, want %.6f", i, p.Lon, want)
		}
	}
}

// TestGreatCircleWrapsTheAntimeridian makes sure longitude comes back wrapped
// rather than running past 180, which would draw a stripe across the map.
func TestGreatCircleWrapsTheAntimeridian(t *testing.T) {
	arc := GreatCircle(geo.Point{Lon: 170, Lat: 0}, 90, 30, 5)
	for i, p := range arc {
		if p.Lon < -180 || p.Lon > 180 {
			t.Errorf("point %d is at %.4fE, outside the map", i, p.Lon)
		}
	}
	if last := arc[len(arc)-1].Lon; math.Abs(last-(-160)) > 1e-9 {
		t.Errorf("arc ends at %.4fE, want -160", last)
	}
}
