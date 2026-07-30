// Package astro holds the sky geometry the map needs: where the sun is, and
// where the auroral oval sits.
package astro

import (
	"math"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

const (
	deg = math.Pi / 180
	rad = 180 / math.Pi
)

// j2000 is the epoch the solar position series is measured from.
var j2000 = time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC)

// daysSinceJ2000 including the fractional part of the day.
func daysSinceJ2000(t time.Time) float64 {
	return t.UTC().Sub(j2000).Seconds() / 86400
}

// SubsolarPoint returns the spot with the sun straight overhead.
//
// Low-precision NOAA series, good to about a hundredth of a degree, which is
// far below one pixel on a 1000px wide world map.
func SubsolarPoint(t time.Time) geo.Point {
	n := daysSinceJ2000(t)

	meanLon := 280.460 + 0.9856474*n
	meanAnom := (357.528 + 0.9856003*n) * deg

	// Ecliptic longitude, with the first two terms of the equation of centre.
	eclipticLon := (meanLon + 1.915*math.Sin(meanAnom) + 0.020*math.Sin(2*meanAnom)) * deg
	obliquity := (23.439 - 0.0000004*n) * deg

	declination := math.Asin(math.Sin(obliquity)*math.Sin(eclipticLon)) * rad
	rightAsc := math.Atan2(
		math.Cos(obliquity)*math.Sin(eclipticLon),
		math.Cos(eclipticLon),
	) * rad

	// Greenwich mean sidereal time, as an angle.
	gmst := 280.46061837 + 360.98564736629*n

	return geo.Point{
		Lon: geo.WrapLon(rightAsc - gmst),
		Lat: declination,
	}
}

// TerminatorLatitude gives the latitude of the day/night boundary at one
// longitude, for a sun at the given declination and subsolar longitude.
func TerminatorLatitude(lon, subsolarLon, declination float64) float64 {
	// At an equinox the declination passes through zero and the boundary goes
	// vertical, so hold it just off zero to keep the tangent finite.
	if math.Abs(declination) < 0.3 {
		declination = math.Copysign(0.3, declination)
		if declination == 0 {
			declination = 0.3
		}
	}
	hourAngle := (lon - subsolarLon) * deg
	return math.Atan(-math.Cos(hourAngle)/math.Tan(declination*deg)) * rad
}

// NightPolygon traces the unlit half of the world.
//
// At any one longitude the sun's altitude is a single sine in latitude, so the
// lit part always reaches one pole and the dark part always reaches the other.
// That makes the night side one band: follow the terminator across the map,
// then close along whichever pole edge is in darkness.
func NightPolygon(subsolar geo.Point, stepDeg float64) []geo.Point {
	var pts []geo.Point
	for lon := -180.0; lon <= 180.0+stepDeg/2; lon += stepDeg {
		lat := TerminatorLatitude(lon, subsolar.Lon, subsolar.Lat)
		pts = append(pts, geo.Point{Lon: lon, Lat: lat})
	}

	// Northern summer lights the north pole, so the darkness hangs south.
	polarLat := -90.0
	if subsolar.Lat < 0 {
		polarLat = 90.0
	}
	pts = append(pts,
		geo.Point{Lon: 180, Lat: polarLat},
		geo.Point{Lon: -180, Lat: polarLat},
	)
	return pts
}
