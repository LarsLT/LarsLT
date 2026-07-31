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
