package render

import (
	"fmt"
	"strings"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// lonX and latY are the projection, re-exported at package level so the layer
// code stays readable.
func lonX(lon float64) float64 { return geo.X(lon) }
func latY(lat float64) float64 { return geo.Y(lat) }

// Sky is everything the renderer needs. Any field may be empty: a source that
// failed simply drops its layer rather than failing the build.
type Sky struct {
	Generated  time.Time
	LandPath   string
	Terminator *Terminator
	Legend     []LegendItem
	Ticker     []string
}

// PolygonPath projects a lon/lat ring into map space, optionally shifted a
// whole turn of the globe so a copy can sit alongside the original.
func PolygonPath(pts []geo.Point, lonShift float64) string {
	if len(pts) == 0 {
		return ""
	}
	xy := make([]geo.XY, 0, len(pts))
	for _, p := range pts {
		xy = append(xy, geo.Project(geo.Point{Lon: p.Lon + lonShift, Lat: p.Lat}))
	}
	return PathD(xy, true)
}

// Document renders the whole SVG.
func Document(sky Sky) string {
	var b strings.Builder
	b.Grow(len(sky.LandPath) + 32*1024)

	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s" role="img" aria-label="%s">`,
		Num(Width), Num(Height), Num(Width), Num(Height), Esc(TitleText))
	fmt.Fprintf(&b, `<title>%s</title>`, Esc(TitleText))
	b.WriteString(`<desc>Day and night on Earth right now, rebuilt every 30 minutes.</desc>`)

	writeDefs(&b)
	writeStyle(&b)

	background(&b)

	// Map group. Everything inside is in 0..1000 x 0..500 map space.
	b.WriteString(`<g clip-path="url(#mapclip)">`)
	ocean(&b)
	graticule(&b)
	if sky.LandPath != "" {
		land(&b, sky.LandPath)
	}
	terminator(&b, sky.Terminator)
	b.WriteString(`</g>`)

	legend(&b, sky.Legend)
	ticker(&b, sky.Ticker)

	b.WriteString(`</svg>`)
	return b.String()
}

func writeDefs(b *strings.Builder) {
	fmt.Fprintf(b, `<defs>`+
		`<linearGradient id="sea" x1="0" y1="0" x2="0" y2="1">`+
		`<stop offset="0" stop-color="%s"/><stop offset="0.5" stop-color="%s"/>`+
		`<stop offset="1" stop-color="%s"/>`+
		`</linearGradient>`+
		`<clipPath id="mapclip"><rect x="0" y="0" width="%s" height="%s"/></clipPath>`+
		`</defs>`,
		OceanPolar, Ocean, OceanPolar, Num(MapW), Num(MapH))
}

// writeStyle emits the one stylesheet. GitHub keeps <style> but strips
// <script>, so every animation in this file lives here.
func writeStyle(b *strings.Builder) {
	fmt.Fprintf(b, `<style>`+
		`text{font-family:%s}`+
		`.mono{font-family:%s}`+
		// One full turn of the Earth, at the speed the Earth actually turns.
		`.night{animation:sweep %ds linear infinite}`+
		`@keyframes sweep{from{transform:translateX(0)}to{transform:translateX(-%spx)}}`+
		`</style>`,
		FontSans, FontMono, int(SolarDay.Seconds()), Num(MapW))
}
