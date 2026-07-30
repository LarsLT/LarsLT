package render

import (
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

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
func Esc(s string) string { return escaper.Replace(strings.Map(xmlSafe, s)) }

// xmlSafe drops the runes XML 1.0 has no encoding for. A control byte in a
// mission name survives JSON decoding, and a parser then refuses the document.
func xmlSafe(r rune) rune {
	switch {
	case r == '\t', r == '\n', r == '\r':
		return r
	case r < 0x20, r == utf8.RuneError,
		r > 0xD7FF && r < 0xE000, r > 0xFFFD && r < 0x10000:
		return -1
	}
	return r
}

// Num formats a float without trailing zeros, which keeps the file small.
func Num(v float64) string { return num(v, 1) }

// Num2 keeps two decimals, for radii and animation delays.
func Num2(v float64) string { return num(v, 2) }

func num(v float64, places int) string {
	// A NaN or Inf coordinate would print literally, and `M NaN,3` breaks the
	// whole image rather than the one shape that produced it.
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
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

// Ellipsis cuts a line down to a width. Feed text arrives unbounded, and SVG
// text neither wraps nor clips itself.
func Ellipsis(s string, size, width float64) string {
	if TextWidth(s, size) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && TextWidth(string(r)+"…", size) > width {
		r = r[:len(r)-1]
	}
	return strings.TrimRight(string(r), " ") + "…"
}
