// Command buildbasemap turns Natural Earth 110m country outlines into the
// committed data/basemap.json. One-off: rerun it only if the map size or the
// simplification tolerance changes. The scheduled build never downloads geometry.
//
//	go run ./cmd/buildbasemap
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/render"
)

// Pinned to a tag, not master, so a rerun reproduces the committed basemap.
const sourceURL = "https://raw.githubusercontent.com/nvkelso/natural-earth-vector/v5.1.2/" +
	"geojson/ne_110m_admin_0_countries.geojson"

// Degrees of lat/lon that may be collapsed away. At 1000px wide one degree of
// longitude is 2.8px, so 0.35 keeps the error around a pixel.
const defaultTolerance = 0.35

// Islands smaller than this in projected pixels are not worth their bytes.
const minFeaturePx = 1.2

type featureCollection struct {
	Features []struct {
		Geometry struct {
			Type        string          `json:"type"`
			Coordinates json.RawMessage `json:"coordinates"`
		} `json:"geometry"`
	} `json:"features"`
}

type basemapFile struct {
	Generated string  `json:"generated"`
	Source    string  `json:"source"`
	Tolerance float64 `json:"tolerance_deg"`
	ViewBox   []int   `json:"viewbox"`
	Rings     int     `json:"rings"`
	Land      string  `json:"land"`
}

func main() {
	tolerance := flag.Float64("tolerance", defaultTolerance, "Douglas-Peucker tolerance in degrees")
	source := flag.String("source", sourceURL, "GeoJSON source URL")
	out := flag.String("out", "data/basemap.json", "output path")
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("buildbasemap: ")

	if err := run(*source, *out, *tolerance); err != nil {
		log.Fatal(err)
	}
}

func run(source, out string, tolerance float64) error {
	rings, err := fetchRings(source)
	if err != nil {
		return err
	}

	var pointsIn int
	var subpaths []string
	for _, ring := range rings {
		pointsIn += len(ring)
		reduced := simplify(unwrap(ring), tolerance)
		if len(reduced) < 4 {
			continue
		}
		minLon, maxLon := lonRange(reduced)
		// A ring that ran past the seam is drawn again one turn over, so the far
		// side of Russia shows up on the opposite map edge.
		for _, shift := range []float64{-360, 0, 360} {
			if maxLon+shift < -180 || minLon+shift > 180 {
				continue
			}
			if d := ringPath(reduced, shift); d != "" {
				subpaths = append(subpaths, d)
			}
		}
	}

	file := basemapFile{
		Generated: time.Now().UTC().Format(time.RFC3339),
		Source:    source,
		Tolerance: tolerance,
		ViewBox:   []int{int(geo.MapW), int(geo.MapH)},
		Rings:     len(subpaths),
		Land:      strings.Join(subpaths, ""),
	}

	encoded, err := json.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := os.WriteFile(out, append(encoded, '\n'), 0o644); err != nil {
		return fmt.Errorf("write: %w", err)
	}

	log.Printf("%d rings / %d points in, %d subpaths out, %.1f KB -> %s",
		len(rings), pointsIn, len(subpaths), float64(len(encoded))/1024, out)
	return nil
}

func fetchRings(source string) ([][]geo.Point, error) {
	client := &http.Client{Timeout: 2 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "space-map-basemap/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch geojson: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch geojson: HTTP %d", resp.StatusCode)
	}

	var fc featureCollection
	if err := json.NewDecoder(resp.Body).Decode(&fc); err != nil {
		return nil, fmt.Errorf("decode geojson: %w", err)
	}

	var rings [][]geo.Point
	for _, feature := range fc.Features {
		switch feature.Geometry.Type {
		case "Polygon":
			var polygon [][][2]float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &polygon); err != nil {
				return nil, fmt.Errorf("decode polygon: %w", err)
			}
			rings = append(rings, toRings(polygon)...)
		case "MultiPolygon":
			var multi [][][][2]float64
			if err := json.Unmarshal(feature.Geometry.Coordinates, &multi); err != nil {
				return nil, fmt.Errorf("decode multipolygon: %w", err)
			}
			for _, polygon := range multi {
				rings = append(rings, toRings(polygon)...)
			}
		}
	}
	return rings, nil
}

func toRings(polygon [][][2]float64) [][]geo.Point {
	out := make([][]geo.Point, 0, len(polygon))
	for _, ring := range polygon {
		points := make([]geo.Point, 0, len(ring))
		for _, c := range ring {
			points = append(points, geo.Point{Lon: c[0], Lat: c[1]})
		}
		out = append(out, points)
	}
	return out
}

// unwrap removes the +-180 seam by letting longitude run continuously. Natural
// Earth stores Russia and Fiji with jumps from 179 to -179, and left alone
// those draw a stripe straight across the map.
func unwrap(ring []geo.Point) []geo.Point {
	if len(ring) == 0 {
		return ring
	}
	out := make([]geo.Point, 0, len(ring))
	out = append(out, ring[0])
	for _, p := range ring[1:] {
		prev := out[len(out)-1].Lon
		for p.Lon-prev > 180 {
			p.Lon -= 360
		}
		for prev-p.Lon > 180 {
			p.Lon += 360
		}
		out = append(out, p)
	}
	return out
}

// simplify is Douglas-Peucker, iterative so a long coastline cannot blow the stack.
func simplify(points []geo.Point, tolerance float64) []geo.Point {
	if len(points) < 3 {
		return points
	}
	keep := make([]bool, len(points))
	keep[0] = true
	keep[len(points)-1] = true

	type span struct{ first, last int }
	stack := []span{{0, len(points) - 1}}

	for len(stack) > 0 {
		s := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		maxDist, index := 0.0, s.first
		for i := s.first + 1; i < s.last; i++ {
			if d := perpendicularDistance(points[i], points[s.first], points[s.last]); d > maxDist {
				maxDist, index = d, i
			}
		}
		if maxDist > tolerance {
			keep[index] = true
			stack = append(stack, span{s.first, index}, span{index, s.last})
		}
	}

	out := points[:0:0]
	for i, p := range points {
		if keep[i] {
			out = append(out, p)
		}
	}
	return out
}

func perpendicularDistance(p, start, end geo.Point) float64 {
	dx, dy := end.Lon-start.Lon, end.Lat-start.Lat
	if dx == 0 && dy == 0 {
		return math.Hypot(p.Lon-start.Lon, p.Lat-start.Lat)
	}
	n := math.Abs(dy*p.Lon - dx*p.Lat + end.Lon*start.Lat - end.Lat*start.Lon)
	return n / math.Hypot(dx, dy)
}

func lonRange(ring []geo.Point) (minLon, maxLon float64) {
	minLon, maxLon = ring[0].Lon, ring[0].Lon
	for _, p := range ring[1:] {
		minLon = math.Min(minLon, p.Lon)
		maxLon = math.Max(maxLon, p.Lon)
	}
	return minLon, maxLon
}

// ringPath projects one ring into an SVG subpath, shifted by whole turns of the globe.
func ringPath(ring []geo.Point, shift float64) string {
	pts := make([]geo.XY, 0, len(ring))
	for _, p := range ring {
		pts = append(pts, geo.Project(geo.Point{Lon: p.Lon + shift, Lat: p.Lat}))
	}

	minX, maxX := pts[0].X, pts[0].X
	minY, maxY := pts[0].Y, pts[0].Y
	for _, p := range pts[1:] {
		minX, maxX = math.Min(minX, p.X), math.Max(maxX, p.X)
		minY, maxY = math.Min(minY, p.Y), math.Max(maxY, p.Y)
	}
	if maxX-minX < minFeaturePx && maxY-minY < minFeaturePx {
		return ""
	}
	// Entirely off-canvas after the shift, so the clip would eat it anyway.
	if maxX < 0 || minX > geo.MapW {
		return ""
	}

	deduped := make([]geo.XY, 0, len(pts))
	for _, p := range pts {
		rounded := geo.XY{X: round1(p.X), Y: round1(p.Y)}
		if n := len(deduped); n > 0 && deduped[n-1] == rounded {
			continue
		}
		deduped = append(deduped, rounded)
	}
	if len(deduped) < 3 {
		return ""
	}
	return render.PathD(deduped, true)
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
