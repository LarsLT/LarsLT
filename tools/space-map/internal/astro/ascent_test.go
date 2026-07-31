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
