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
	Generated time.Time
	LandPath  string
	Legend    []LegendItem
	Ticker    []string
}

// Document renders the whole SVG.
func Document(sky Sky) string {
	var b strings.Builder
	b.Grow(len(sky.LandPath) + 32*1024)

	fmt.Fprintf(&b,
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %s %s" width="%s" height="%s" role="img" aria-label="%s">`,
		Num(Width), Num(Height), Num(Width), Num(Height), Esc(TitleText))
	fmt.Fprintf(&b, `<title>%s</title>`, Esc(TitleText))
	fmt.Fprintf(&b, `<desc>Generated %s. Rebuilt every 30 minutes by GitHub Actions.</desc>`,
		Esc(sky.Generated.Format(time.RFC3339)))

	writeDefs(&b)
	writeStyle(&b)

	background(&b)
	starfield(&b)

	// Map group. Everything inside is in 0..1000 x 0..500 map space.
	fmt.Fprintf(&b, `<g transform="translate(0,%s)" clip-path="url(#mapclip)">`, Num(MapY))
	graticule(&b)
	if sky.LandPath != "" {
		land(&b, sky.LandPath)
	}
	b.WriteString(`</g>`)

	header(&b, sky.Generated.UTC().Format("2006-01-02 15:04 UTC"))
	legend(&b, sky.Legend)
	ticker(&b, sky.Ticker)

	b.WriteString(`</svg>`)
	return b.String()
}

func writeDefs(b *strings.Builder) {
	fmt.Fprintf(b, `<defs>`+
		`<linearGradient id="sky" x1="0" y1="0" x2="0" y2="1">`+
		`<stop offset="0" stop-color="%s"/><stop offset="1" stop-color="%s"/>`+
		`</linearGradient>`+
		`<clipPath id="mapclip"><rect x="0" y="0" width="%s" height="%s"/></clipPath>`+
		`</defs>`,
		BgTop, BgBottom, Num(MapW), Num(MapH))
}

// writeStyle emits the one stylesheet. GitHub keeps <style> but strips
// <script>, so every animation in this file lives here.
func writeStyle(b *strings.Builder) {
	fmt.Fprintf(b, `<style>`+
		`text{font-family:%s}`+
		`.mono{font-family:%s}`+
		`.tw1,.tw2,.tw3{animation-iteration-count:infinite;animation-timing-function:ease-in-out}`+
		`.tw1{animation-name:twinkle1;animation-duration:3.1s}`+
		`.tw2{animation-name:twinkle2;animation-duration:4.9s}`+
		`.tw3{animation-name:twinkle3;animation-duration:7.3s}`+
		`@keyframes twinkle1{0%%,100%%{opacity:.9}50%%{opacity:.35}}`+
		`@keyframes twinkle2{0%%,100%%{opacity:.6}50%%{opacity:.22}}`+
		`@keyframes twinkle3{0%%,100%%{opacity:.34}50%%{opacity:.12}}`+
		`</style>`,
		FontSans, FontMono)
}
