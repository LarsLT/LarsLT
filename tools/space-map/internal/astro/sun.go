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

// SubsolarPoint returns the spot with the sun straight overhead. The NOAA
// low-precision series, good to well under a pixel at this map's scale.
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

	return geo.Point{
		Lon: geo.WrapLon(rightAsc - GreenwichAngle(t)),
		Lat: declination,
	}
}

// GreenwichAngle is Greenwich mean sidereal time as an angle in degrees, which
// is how far the Earth has turned under the stars.
func GreenwichAngle(t time.Time) float64 {
	return 280.46061837 + 360.98564736629*daysSinceJ2000(t)
}

// SunElevation is how far the sun stands above the horizon at a point, in
// degrees. Below zero is night, the only time an aurora can be seen.
func SunElevation(p, subsolar geo.Point) float64 {
	cosZenith := math.Sin(p.Lat*deg)*math.Sin(subsolar.Lat*deg) +
		math.Cos(p.Lat*deg)*math.Cos(subsolar.Lat*deg)*math.Cos((p.Lon-subsolar.Lon)*deg)
	return math.Asin(math.Min(1, math.Max(-1, cosZenith))) * rad
}

// AnyDark reports whether any of these points has the sun that far under the
// horizon. A shape that is nowhere dark is one nobody on Earth can see.
func AnyDark(track []geo.Point, subsolar geo.Point, depression float64) bool {
	for _, p := range track {
		if SunElevation(p, subsolar) < -depression {
			return true
		}
	}
	return false
}

// TerminatorLatitude gives the latitude of the day/night boundary at one
// longitude, for a sun at the given declination and subsolar longitude.
func TerminatorLatitude(lon, subsolarLon, declination float64) float64 {
	// At an equinox the declination passes through zero and the boundary goes
	// vertical, so hold it just off zero to keep the tangent finite.
	if math.Abs(declination) < 0.3 {
		declination = math.Copysign(0.3, declination)
	}
	hourAngle := (lon - subsolarLon) * deg
	return math.Atan(-math.Cos(hourAngle)/math.Tan(declination*deg)) * rad
}

// TerminatorTrace follows the day/night boundary alone, west to east. It is the
// only part of the night's outline that is a real line on the ground.
func TerminatorTrace(subsolar geo.Point, stepDeg float64) []geo.Point {
	var pts []geo.Point
	for lon := -180.0; lon <= 180.0+stepDeg/2; lon += stepDeg {
		lat := TerminatorLatitude(lon, subsolar.Lon, subsolar.Lat)
		pts = append(pts, geo.Point{Lon: lon, Lat: lat})
	}
	return pts
}

// DarkPolygon traces the ground where the sun is at least depression degrees
// under the horizon, which is a cap around the antisolar point.
func DarkPolygon(subsolar geo.Point, depression, stepDeg float64) []geo.Point {
	antisolar := geo.Point{Lat: -subsolar.Lat, Lon: geo.WrapLon(subsolar.Lon + 180)}
	radius := 90 - depression

	// Deep into a season the cap swallows a pole and closes along it, the way
	// night does. Near an equinox it reaches neither pole and is a lens.
	polarLat, holdsPole := capHoldsPole(antisolar, radius)
	if !holdsPole {
		return capLens(antisolar, radius, stepDeg)
	}

	var ring []geo.Point
	for lon := -180.0; lon <= 180.0+stepDeg/2; lon += stepDeg {
		lats := capCrossings(antisolar, radius, lon)
		if len(lats) == 0 {
			return nil
		}
		ring = append(ring, geo.Point{Lon: lon, Lat: lats[0]})
	}
	return append(ring,
		geo.Point{Lon: 180, Lat: polarLat},
		geo.Point{Lon: -180, Lat: polarLat},
	)
}

// capHoldsPole says which pole a cap encloses, if either.
func capHoldsPole(centre geo.Point, radius float64) (float64, bool) {
	switch {
	case 90-centre.Lat < radius:
		return 90, true
	case 90+centre.Lat < radius:
		return -90, true
	}
	return 0, false
}

// capLens traces a cap that reaches no pole, far edge out and near edge back.
// Longitudes are left unwrapped so the shape stays whole across the seam.
func capLens(centre geo.Point, radius, stepDeg float64) []geo.Point {
	var near, far []geo.Point
	for lon := centre.Lon - 180; lon <= centre.Lon+180; lon += stepDeg {
		lats := capCrossings(centre, radius, lon)
		if len(lats) < 2 {
			continue
		}
		near = append(near, geo.Point{Lon: lon, Lat: math.Max(lats[0], lats[1])})
		far = append(far, geo.Point{Lon: lon, Lat: math.Min(lats[0], lats[1])})
	}
	for i := len(far) - 1; i >= 0; i-- {
		near = append(near, far[i])
	}
	return near
}

// capCrossings is where a circle of some angular radius about a centre crosses
// one meridian. A meridian that misses it altogether gets nothing back.
func capCrossings(centre geo.Point, radius, lon float64) []float64 {
	a := math.Sin(centre.Lat * deg)
	b := math.Cos(centre.Lat*deg) * math.Cos((lon-centre.Lon)*deg)
	amplitude := math.Hypot(a, b)
	if amplitude == 0 {
		return nil
	}
	ratio := math.Cos(radius*deg) / amplitude
	if math.Abs(ratio) > 1 {
		return nil
	}

	phase := math.Atan2(b, a)
	principal := math.Asin(ratio)
	var lats []float64
	for _, root := range [2]float64{principal - phase, math.Pi - principal - phase} {
		if lat := wrapAngle(root) * rad; math.Abs(lat) <= 90 {
			lats = append(lats, lat)
		}
	}
	return lats
}

// NightPolygon traces the unlit half of the world. Darkness always reaches one
// pole, so it is a single band: the terminator, closed along that pole's edge.
func NightPolygon(subsolar geo.Point, stepDeg float64) []geo.Point {
	// Northern summer lights the north pole, so the darkness hangs south.
	polarLat := -90.0
	if subsolar.Lat < 0 {
		polarLat = 90.0
	}
	return append(TerminatorTrace(subsolar, stepDeg),
		geo.Point{Lon: 180, Lat: polarLat},
		geo.Point{Lon: -180, Lat: polarLat},
	)
}
