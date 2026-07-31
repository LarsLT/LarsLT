package astro

import (
	"math"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// LaunchAzimuth is the bearing flown to reach an inclination from a pad, degrees
// clockwise from north. Always the prograde root, so some sites draw backwards.
func LaunchAzimuth(padLat, inclination float64) float64 {
	// Below the pad latitude no direct ascent reaches the inclination at all.
	// Clamping lands on due east, the closest it can get, instead of NaN.
	ratio := math.Cos(inclination*deg) / math.Cos(padLat*deg)
	ratio = math.Min(1, math.Max(-1, ratio))

	azimuth := math.Asin(ratio) * rad
	if inclination > 90 {
		azimuth = 180 - azimuth
	}
	return math.Mod(azimuth+360, 360)
}

// MirrorAzimuth is the other bearing that reaches the same inclination. Both
// roots are equally valid physics; only range safety picks between them.
func MirrorAzimuth(azimuth float64) float64 {
	return math.Mod(540-azimuth, 360)
}

// Corridor is the span of azimuths a range allows, running clockwise from From
// to To. It may wrap through north, which is how an easterly range is written.
type Corridor struct{ From, To float64 }

// Contains says whether a bearing is one this range would let fly.
func (c Corridor) Contains(azimuth float64) bool {
	span := math.Mod(c.To-c.From+360, 360)
	return math.Mod(azimuth-c.From+360, 360) <= span
}

// Nearest is the closest bearing the range allows, which is the bearing itself
// whenever it already fits.
func (c Corridor) Nearest(azimuth float64) float64 {
	if c.Contains(azimuth) {
		return azimuth
	}
	if apart(azimuth, c.From) <= apart(azimuth, c.To) {
		return c.From
	}
	return c.To
}

// apart is the angle between two bearings, never more than half a turn.
func apart(a, b float64) float64 {
	gap := math.Mod(math.Abs(a-b), 360)
	return math.Min(gap, 360-gap)
}

// GreatCircle walks arcDeg of great circle from a point along a bearing, one
// sample every stepDeg. The first sample is the starting point.
func GreatCircle(from geo.Point, azimuth, arcDeg, stepDeg float64) []geo.Point {
	if stepDeg <= 0 || arcDeg <= 0 {
		return nil
	}

	lat := from.Lat * deg
	sinLat, cosLat := math.Sincos(lat)
	sinAz, cosAz := math.Sincos(azimuth * deg)

	var out []geo.Point
	for travelled := 0.0; travelled <= arcDeg+1e-9; travelled += stepDeg {
		sinD, cosD := math.Sincos(travelled * deg)

		nextLat := math.Asin(sinLat*cosD + cosLat*sinD*cosAz)
		dLon := math.Atan2(sinAz*sinD*cosLat, cosD-sinLat*math.Sin(nextLat))

		out = append(out, geo.Point{
			Lon: geo.WrapLon(from.Lon + dLon*rad),
			Lat: nextLat * rad,
		})
	}
	return out
}
