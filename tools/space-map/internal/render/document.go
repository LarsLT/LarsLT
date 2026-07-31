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
	Aurora     []Aurora
	Eclipse    *Eclipse
	Station    *Station
	Ascent     *Ascent
	Launches   []LaunchPad
	Legend     []LegendItem
	Ticker     []string
}

// PolygonPath projects a lon/lat ring into map space and closes it.
func PolygonPath(pts []geo.Point) string {
	if len(pts) == 0 {
		return ""
	}
	xy := make([]geo.XY, 0, len(pts))
	for _, p := range pts {
		xy = append(xy, geo.Project(p))
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
	// Stamping the build time is the only way to tell a stale image from a
	// frozen workflow, since camo serves whatever it last cached either way.
	if sky.Generated.IsZero() {
		b.WriteString(`<desc>Day and night on Earth right now, rebuilt every 30 minutes.</desc>`)
	} else {
		fmt.Fprintf(&b, `<desc>Day and night on Earth at %s UTC, rebuilt every 30 minutes.</desc>`,
			Esc(sky.Generated.UTC().Format("2006-01-02 15:04")))
	}

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
	// No glow to hold on the dark side, no reason to carry a second copy of it.
	masked := len(sky.Aurora) > 0 && sky.Terminator != nil && sky.Terminator.NightPath != ""
	if masked {
		nightMask(&b, sky.Terminator)
	}
	aurora(&b, sky.Aurora, masked)
	eclipse(&b, sky.Eclipse)
	station(&b, sky.Station)
	ascent(&b, sky.Ascent)
	launchPads(&b, sky.Launches)
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
		// Aurora does not stop dead at the terminator, and neither should the
		// mask that holds it there.
		`<filter id="dusk" x="-5%%" y="-5%%" width="110%%" height="110%%">`+
		`<feGaussianBlur stdDeviation="5"/></filter>`+
		`</defs>`,
		OceanPolar, Ocean, OceanPolar,
		Num(MapW), Num(MapH))
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
		`.ping{animation:ping 2.6s ease-out infinite}`+
		`@keyframes ping{from{transform:scale(.5);opacity:.9}to{transform:scale(3.6);opacity:0}}`+
		`.glow{animation:glow 4s ease-in-out infinite}`+
		`@keyframes glow{0%%,100%%{opacity:.42}50%%{opacity:.78}}`+
		`.umbra{offset-rotate:0deg;animation-name:umbra;animation-timing-function:linear;`+
		`animation-iteration-count:infinite}`+
		`@keyframes umbra{0%%{offset-distance:0%%;opacity:0}10%%,90%%{opacity:.95}`+
		`100%%{offset-distance:100%%;opacity:0}}`+
		// The station rides its leg at the speed it actually flies, and the
		// negative delay drops it where it is now rather than at the start.
		`.station{offset-rotate:0deg;animation-name:ride;animation-timing-function:linear;`+
		`animation-iteration-count:infinite}`+
		`@keyframes ride{from{offset-distance:0%%}to{offset-distance:100%%}}`+
		// One climb per load, not a loop: a launch happens once, and a dot going
		// round forever would claim a rocket leaves every few seconds.
		`.ascent{offset-rotate:0deg;animation-name:ascent;animation-timing-function:ease-in;`+
		`animation-iteration-count:1;animation-fill-mode:forwards}`+
		`@keyframes ascent{0%%{offset-distance:0%%;opacity:0}15%%,85%%{opacity:.9}`+
		`100%%{offset-distance:100%%;opacity:0}}`+
		`</style>`,
		FontSans, FontMono, int(SolarDay.Seconds()), Num(MapW))
}
