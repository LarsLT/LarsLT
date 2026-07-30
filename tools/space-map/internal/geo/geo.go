// Package geo holds the equirectangular projection shared by the generators
// and the renderer. Map space is 1000x500, matching data/basemap.json.
package geo

import "math"

const (
	MapW = 1000.0
	MapH = 500.0
)

// Point is a longitude/latitude pair in degrees.
type Point struct {
	Lon, Lat float64
}

// XY is a projected point in map space.
type XY struct {
	X, Y float64
}

func X(lon float64) float64 { return (lon + 180) / 360 * MapW }

func Y(lat float64) float64 { return (90 - lat) / 180 * MapH }

func Project(p Point) XY { return XY{X(p.Lon), Y(p.Lat)} }

// WrapLon folds any longitude back into -180..180.
func WrapLon(lon float64) float64 {
	l := math.Mod(lon+180, 360)
	if l < 0 {
		l += 360
	}
	return l - 180
}

// SplitAntimeridian breaks a track into runs that do not jump across the map.
// Without it an orbit or an eclipse path draws a line straight back across the
// world every time it passes the seam.
func SplitAntimeridian(track []Point) [][]Point {
	var runs [][]Point
	var current []Point
	for _, p := range track {
		p.Lon = WrapLon(p.Lon)
		if len(current) > 0 && math.Abs(p.Lon-current[len(current)-1].Lon) > 180 {
			runs = append(runs, current)
			current = nil
		}
		current = append(current, p)
	}
	if len(current) > 0 {
		runs = append(runs, current)
	}
	return runs
}
