package render

import (
	"strconv"
	"strings"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

var escaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
)

// Esc makes text safe to drop between SVG tags or inside an attribute.
func Esc(s string) string { return escaper.Replace(s) }

// Num formats a float without trailing zeros, which keeps the file small.
func Num(v float64) string { return num(v, 1) }

// Num2 keeps two decimals, for radii and animation delays.
func Num2(v float64) string { return num(v, 2) }

func num(v float64, places int) string {
	s := strconv.FormatFloat(v, 'f', places, 64)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimSuffix(s, ".")
	}
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

// PathD builds a path from projected points.
func PathD(pts []geo.XY, close bool) string {
	if len(pts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('M')
	for i, p := range pts {
		if i > 0 {
			b.WriteByte('L')
		}
		b.WriteString(Num(p.X))
		b.WriteByte(',')
		b.WriteString(Num(p.Y))
	}
	if close {
		b.WriteByte('Z')
	}
	return b.String()
}

// TrackD projects a lon/lat track and returns one path per antimeridian run,
// so nothing draws a stripe across the map.
func TrackD(track []geo.Point) []string {
	var out []string
	for _, run := range geo.SplitAntimeridian(track) {
		if len(run) < 2 {
			continue
		}
		pts := make([]geo.XY, 0, len(run))
		for _, p := range run {
			pts = append(pts, geo.Project(p))
		}
		out = append(out, PathD(pts, false))
	}
	return out
}

// TextWidth is a rough advance width, good enough to lay out legend chips.
func TextWidth(s string, size float64) float64 {
	return float64(len([]rune(s))) * size * 0.55
}
