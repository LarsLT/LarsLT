package astro

import (
	"math"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// The geomagnetic north pole for the current epoch. It drifts about a tenth of
// a degree a year, so a hardcoded pair stays good for the life of this map.
var geomagneticNorth = geo.Point{Lat: 80.7, Lon: -72.7}

const (
	// quietBoundary is the equatorward edge of the auroral oval at Kp 0, in
	// geomagnetic latitude, and it drops by boundaryPerKp for each Kp step.
	quietBoundary = 66.5
	boundaryPerKp = 2.0

	// ovalWidth is how far poleward the glow reaches. Inside it lies the polar
	// cap, which is dark.
	ovalWidth = 7.0

	// HorizonSkirt is how far south the glow is seen from. Aurora sits about
	// 100 km up, so it clears the horizon a few hundred kilometres away.
	HorizonSkirt = 5.5
)

// OvalBoundary is the geomagnetic latitude of the equatorward edge of the oval
// at a given Kp.
func OvalBoundary(kp float64) float64 {
	return quietBoundary - boundaryPerKp*kp
}

// VisibleFrom is the lowest geomagnetic latitude that can see the glow at all,
// low on the horizon.
func VisibleFrom(kp float64) float64 {
	return OvalBoundary(kp) - HorizonSkirt
}

// Oval traces the ground one hemisphere's aurora is seen from, from a Kp alone.
// It is the fallback for when the forecast grid cannot be had.
func Oval(kp float64, north bool, stepDeg float64) []geo.Point {
	pole := geomagneticNorth
	seen := VisibleFrom(kp)
	if !north {
		pole = geo.Point{Lat: -pole.Lat, Lon: geo.WrapLon(pole.Lon + 180)}
		seen = -seen
	}

	var ring []geo.Point
	for lon := -180.0; lon <= 180.0+stepDeg/2; lon += stepDeg {
		ring = append(ring, geo.Point{Lon: lon, Lat: smallCircleLat(pole, seen, lon)})
	}

	poleward := seen + HorizonSkirt + ovalWidth
	if !north {
		poleward = seen - HorizonSkirt - ovalWidth
	}
	for lon := 180.0; lon >= -180.0-stepDeg/2; lon -= stepDeg {
		ring = append(ring, geo.Point{Lon: lon, Lat: smallCircleLat(pole, poleward, lon)})
	}
	return ring
}

// smallCircleLat is where the circle of constant geomagnetic latitude crosses
// one meridian: one sinusoid in lat, single-rooted while magneticLat < pole.Lat.
func smallCircleLat(pole geo.Point, magneticLat, lon float64) float64 {
	radius := (90 - math.Abs(magneticLat)) * deg

	a := math.Sin(pole.Lat * deg)
	b := math.Cos(pole.Lat*deg) * math.Cos((lon-pole.Lon)*deg)
	amplitude := math.Hypot(a, b)
	phase := math.Atan2(b, a)

	// Past that precondition some meridians miss the circle entirely. Clamping
	// the ratio would answer them with a latitude nowhere near it.
	ratio := math.Cos(radius) / amplitude
	if math.Abs(ratio) > 1 {
		return math.Copysign(90, pole.Lat)
	}
	principal := math.Asin(ratio)

	// The sinusoid has two roots. Only one is a real latitude when the circle
	// encloses the pole, and reflecting the other would land a meridian away.
	for _, root := range [2]float64{principal - phase, math.Pi - principal - phase} {
		if lat := wrapAngle(root) * rad; math.Abs(lat) <= 90 {
			return lat
		}
	}
	return math.Copysign(90, pole.Lat)
}

// wrapAngle folds radians into -pi..pi.
func wrapAngle(a float64) float64 {
	a = math.Mod(a+math.Pi, 2*math.Pi)
	if a < 0 {
		a += 2 * math.Pi
	}
	return a - math.Pi
}

// GeographicLatAt is the geographic latitude a geomagnetic latitude sits at
// over one meridian, which is how the oval gets described in plain words.
func GeographicLatAt(magneticLat, lon float64, north bool) float64 {
	pole := geomagneticNorth
	if !north {
		pole = geo.Point{Lat: -pole.Lat, Lon: geo.WrapLon(pole.Lon + 180)}
		magneticLat = -magneticLat
	}
	return smallCircleLat(pole, magneticLat, lon)
}
