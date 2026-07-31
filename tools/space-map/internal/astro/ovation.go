package astro

import (
	"fmt"
	"math"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

const (
	gridLons = 360
	gridLats = 181
)

// visibleProbability is the contour the drawn band stops at. OVATION's diffuse
// edge is real aurora, and a high threshold would drop the faint horizon arcs.
const visibleProbability = 3.0

// AuroraGrid is NOAA's OVATION output: the chance of aurora overhead at every
// whole degree, forecast an hour past the measurements behind it.
type AuroraGrid struct {
	Forecast time.Time
	// prob is [lon 0..359 east][lat counted up from the south pole]. Cells start
	// NaN so half a grid cannot pass for a quiet sky.
	prob [gridLons][gridLats]float64
}

// NewAuroraGrid returns an empty grid for a forecast time, to be filled in cell
// by cell and then checked with Complete.
func NewAuroraGrid(forecast time.Time) *AuroraGrid {
	g := &AuroraGrid{Forecast: forecast}
	for lon := range g.prob {
		for lat := range g.prob[lon] {
			g.prob[lon][lat] = math.NaN()
		}
	}
	return g
}

// Set records one cell, addressed the way SWPC publishes it: longitude east of
// Greenwich, latitude from the south pole up, probability in percent.
func (g *AuroraGrid) Set(lon, lat int, probability float64) error {
	if lon < 0 || lon >= gridLons || lat < -90 || lat > 90 {
		return fmt.Errorf("cell %d,%d is off the grid", lon, lat)
	}
	if math.IsNaN(probability) || probability < 0 || probability > 100 {
		return fmt.Errorf("cell %d,%d has probability %g", lon, lat, probability)
	}
	g.prob[lon][lat+90] = probability
	return nil
}

// Complete reports whether every cell arrived. A gap would read as no aurora
// there, which is a hole cut in the oval rather than a missing value.
func (g *AuroraGrid) Complete() error {
	for lon := range g.prob {
		for lat := range g.prob[lon] {
			if math.IsNaN(g.prob[lon][lat]) {
				return fmt.Errorf("no value for longitude %d latitude %d", lon, lat-90)
			}
		}
	}
	return nil
}

// Footprint traces the ground one hemisphere's aurora is seen from: the oval
// plus its horizon skirt, in one run per unbroken stretch of meridians.
func (g *AuroraGrid) Footprint(north bool) [][]geo.Point {
	edges := make([]auroraEdge, 0, gridLons)
	for i := range gridLons {
		edges = append(edges, g.edgeAt(float64(i)-180, north))
	}
	return ringsFrom(edges)
}

// ReachAt is the lowest latitude the glow clears the horizon at over one
// meridian, and whether there is anything to see there at all.
func (g *AuroraGrid) ReachAt(lon float64, north bool) (float64, bool) {
	e := g.edgeAt(lon, north)
	return e.seen, e.present
}

// auroraEdge is one meridian's slice of the band: where the glow can be seen
// from, and where it stops on the way to the dark polar cap.
type auroraEdge struct {
	lon     float64
	seen    float64
	pole    float64
	present bool
}

// edgeAt reads one column, growing out from its strongest cell so that a stray
// reading far south cannot drag the whole band down over Spain.
func (g *AuroraGrid) edgeAt(lon float64, north bool) auroraEdge {
	col := int(geo.WrapLon(lon) + 360)
	col %= gridLons
	dir := 1
	if !north {
		dir = -1
	}

	peak := 0
	for step := range 91 {
		if lat := dir * step; g.prob[col][lat+90] > g.prob[col][peak+90] {
			peak = lat
		}
	}
	if g.prob[col][peak+90] < visibleProbability {
		return auroraEdge{lon: lon}
	}

	low, high := peak, peak
	for lat := peak - dir; lat*dir >= 0 && g.prob[col][lat+90] >= visibleProbability; lat -= dir {
		low = lat
	}
	for lat := peak + dir; lat*dir <= 90 && g.prob[col][lat+90] >= visibleProbability; lat += dir {
		high = lat
	}

	// Aurora stands about 100 km up, so it is seen from well equatorward of
	// where it hangs. That skirt is the whole point of the band.
	seen := float64(low) - float64(dir)*HorizonSkirt
	if seen*float64(dir) < 0 {
		seen = 0
	}
	return auroraEdge{lon: lon, seen: seen, pole: float64(high), present: true}
}

// ringsFrom joins the meridian slices into rings. A band reaching every
// meridian repeats its first slice past the seam; a partial one is cut there.
func ringsFrom(edges []auroraEdge) [][]geo.Point {
	full := true
	for _, e := range edges {
		if !e.present {
			full = false
			break
		}
	}
	if full {
		wrap := edges[0]
		wrap.lon += 360
		return [][]geo.Point{ringOf(append(edges, wrap))}
	}

	var rings [][]geo.Point
	var run []auroraEdge
	for _, e := range edges {
		if e.present {
			run = append(run, e)
			continue
		}
		if len(run) > 1 {
			rings = append(rings, ringOf(run))
		}
		run = nil
	}
	if len(run) > 1 {
		rings = append(rings, ringOf(run))
	}
	return rings
}

// ringOf walks the seen edge west to east and the poleward edge back, the same
// way the Kp oval is drawn.
func ringOf(run []auroraEdge) []geo.Point {
	ring := make([]geo.Point, 0, 2*len(run))
	for _, e := range run {
		ring = append(ring, geo.Point{Lon: e.lon, Lat: e.seen})
	}
	for i := len(run) - 1; i >= 0; i-- {
		ring = append(ring, geo.Point{Lon: run[i].lon, Lat: run[i].pole})
	}
	return ring
}
