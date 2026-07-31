package render

import (
	"fmt"
	"strings"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// LegendItem is one coloured chip under the map.
type LegendItem struct {
	Colour string
	Label  string
}

// Terminator is the unlit half of the world, already projected. The shape is
// baked at the current sun position and then slides west, as the real one does.
type Terminator struct {
	NightPath string
	// DarkPath is the smaller region where the sky is dark enough to see a faint
	// aurora, well inside the night. Empty falls back to plain night.
	DarkPath string
	// EdgePath is the day/night boundary alone. NightPath has to close along the
	// dark pole, and stroking that seam draws a terminator where there is none.
	EdgePath string
	SunX     float64
	SunY     float64
}

// terminator draws the night side twice, side by side, so that sliding the
// group one full map width west loops without a seam.
func terminator(b *strings.Builder, term *Terminator) {
	if term == nil || term.NightPath == "" {
		return
	}
	b.WriteString(`<g class="night">`)
	for _, shift := range []float64{0, MapW} {
		fmt.Fprintf(b, `<g transform="translate(%s,0)">`, Num(shift))
		if term.EdgePath == "" {
			fmt.Fprintf(b,
				`<path d="%s" fill="%s" fill-opacity="0.55" stroke="%s" stroke-width="0.8" stroke-opacity="0.35"/>`,
				term.NightPath, Night, Accent)
		} else {
			fmt.Fprintf(b, `<path d="%s" fill="%s" fill-opacity="0.55"/>`, term.NightPath, Night)
			fmt.Fprintf(b,
				`<path d="%s" fill="none" stroke="%s" stroke-width="0.8" stroke-opacity="0.35"/>`,
				term.EdgePath, Accent)
		}
		sun(b, term.SunX, term.SunY)
		b.WriteString(`</g>`)
	}
	b.WriteString(`</g>`)
}

// sun marks the spot with the sun straight overhead.
func sun(b *strings.Builder, x, y float64) {
	fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="9" fill="%s" opacity="0.10"/>`,
		Num(x), Num(y), SunGlow)
	fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="4.5" fill="%s" opacity="0.22"/>`,
		Num(x), Num(y), SunGlow)
	fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="2.2" fill="%s"/>`, Num(x), Num(y), SunCore)
}

func background(b *strings.Builder) {
	fmt.Fprintf(b, `<rect width="%s" height="%s" fill="%s"/>`, Num(Width), Num(Height), BgBottom)
}

// ocean makes the Earth opaque. Without it the sea is a hole showing whatever
// sits behind the map.
func ocean(b *strings.Builder) {
	fmt.Fprintf(b, `<rect width="%s" height="%s" fill="url(#sea)"/>`, Num(MapW), Num(MapH))
}

func graticule(b *strings.Builder) {
	fmt.Fprintf(b, `<g stroke="%s" stroke-width="0.6">`, Graticule)
	for lon := -150.0; lon < 180; lon += 30 {
		x := Num(lonX(lon))
		fmt.Fprintf(b, `<line x1="%s" y1="0" x2="%s" y2="%s"/>`, x, x, Num(MapH))
	}
	for lat := -60.0; lat < 90; lat += 30 {
		if lat == 0 {
			continue
		}
		y := Num(latY(lat))
		fmt.Fprintf(b, `<line x1="0" y1="%s" x2="%s" y2="%s"/>`, y, Num(MapW), y)
	}
	b.WriteString(`</g>`)
	eq := Num(latY(0))
	fmt.Fprintf(b,
		`<line x1="0" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1" stroke-dasharray="3 4"/>`,
		eq, Num(MapW), eq, Graticule)
}

// nightMaskID names the mask that keeps the aurora on the dark side.
const nightMaskID = "afterdark"

// nightMask is the night side again, painted white, as a mask. It carries the
// same sweep, so the glow and the darkness it needs never drift apart.
func nightMask(b *strings.Builder, term *Terminator) {
	if term == nil || term.NightPath == "" {
		return
	}
	dark := term.DarkPath
	if dark == "" {
		dark = term.NightPath
	}
	fmt.Fprintf(b, `<mask id="%s" maskUnits="userSpaceOnUse" x="0" y="0" width="%s" height="%s">`,
		nightMaskID, Num(MapW), Num(MapH))
	// Blurring the copies together rather than one by one: separately, the soft
	// edges would not meet and a seam would slide across the map. Three of them,
	// since near an equinox the shape is a lens that hangs over the seam.
	fmt.Fprintf(b, `<g class="night" filter="url(#dusk)">`)
	for _, shift := range []float64{-MapW, 0, MapW} {
		fmt.Fprintf(b, `<path d="%s" transform="translate(%s,0)" fill="#fff"/>`, dark, Num(shift))
	}
	b.WriteString(`</g></mask>`)
}

// Fade is how strong the glow is over one meridian, from 0 to 1.
type Fade struct {
	Lon   float64
	Value float64
}

// Aurora is one hemisphere's glow band, already projected and closed.
type Aurora struct {
	Path  string
	North bool
	// Fade is the strength profile along the band, west to east. Empty leaves it
	// evenly lit, which is what the Kp fallback knows.
	Fade []Fade
	// Skirt is how much of the band's height is ground the glow is only seen
	// from, low on the horizon, rather than ground it stands over.
	Skirt float64
}

// aurora draws the bands inside the night mask, so they show only over the part
// of the world dark enough to see them from.
func aurora(b *strings.Builder, bands []Aurora, masked bool) {
	if len(bands) == 0 {
		return
	}
	if masked {
		fmt.Fprintf(b, `<g mask="url(#%s)">`, nightMaskID)
	}
	for i, band := range bands {
		if band.Path == "" {
			continue
		}
		gradient := fmt.Sprintf("aurora%d", i)
		auroraGradient(b, gradient, band)

		fade := ""
		if len(band.Fade) > 1 {
			id := fmt.Sprintf("alongoval%d", i)
			auroraFade(b, id, band.Fade)
			fade = fmt.Sprintf(` mask="url(#%s)"`, id)
		}
		fmt.Fprintf(b, `<path class="glow" d="%s" fill="url(#%s)" filter="url(#haze)"%s/>`,
			band.Path, gradient, fade)
	}
	if masked {
		b.WriteString(`</g>`)
	}
}

// auroraFade masks a band along its length. The oval is brightest around
// midnight, and without this an arc switches on at the meridian it was cut at.
func auroraFade(b *strings.Builder, id string, fade []Fade) {
	x1, x2 := geo.X(fade[0].Lon), geo.X(fade[len(fade)-1].Lon)
	if x2-x1 < 1 {
		return
	}
	fmt.Fprintf(b, `<linearGradient id="%sramp" gradientUnits="userSpaceOnUse"`+
		` x1="%s" y1="0" x2="%s" y2="0">`, id, Num(x1), Num(x2))
	for _, f := range fade {
		fmt.Fprintf(b, `<stop offset="%s" stop-color="#fff" stop-opacity="%s"/>`,
			Num2((geo.X(f.Lon)-x1)/(x2-x1)), Num2(f.Value))
	}
	b.WriteString(`</linearGradient>`)
	fmt.Fprintf(b, `<mask id="%s" maskUnits="userSpaceOnUse" x="0" y="0" width="%s" height="%s">`+
		`<rect x="0" y="0" width="%s" height="%s" fill="url(#%sramp)"/></mask>`,
		id, Num(MapW), Num(MapH), Num(MapW), Num(MapH), id)
}

// auroraGradient puts the band's bright arc where the glow actually hangs and
// fades it out across the skirt, which is only a glow on someone's horizon.
func auroraGradient(b *strings.Builder, id string, band Aurora) {
	arc := 1 - min(max(band.Skirt, 0.02), 0.98)
	if !band.North {
		arc = 1 - arc
	}
	stops := []struct {
		at      float64
		colour  string
		opacity float64
	}{
		{0, AuroraEdge, 0.22},
		{arc, AuroraCore, 0.7},
		{1, AuroraCore, 0.04},
	}
	if !band.North {
		stops[0], stops[2] = stops[2], stops[0]
		stops[0].at, stops[2].at = 0, 1
	}

	fmt.Fprintf(b, `<linearGradient id="%s" x1="0" y1="0" x2="0" y2="1">`, id)
	for _, s := range stops {
		fmt.Fprintf(b, `<stop offset="%s" stop-color="%s" stop-opacity="%s"/>`,
			Num2(s.at), s.colour, Num2(s.opacity))
	}
	b.WriteString(`</linearGradient>`)
}

// Eclipse is the shadow's track: a filled band between the two limits, and the
// centre line the umbra runs along. Which eclipse it is goes in the ticker.
type Eclipse struct {
	Band string
	// Centre is one path per crossing of the map's edge, and Umbra is the run
	// long enough to be worth riding.
	Centre  []string
	Umbra   string
	Seconds int
}

func eclipse(b *strings.Builder, e *Eclipse) {
	if e == nil || len(e.Centre) == 0 {
		return
	}
	if e.Band != "" {
		fmt.Fprintf(b, `<path d="%s" fill="%s" opacity="0.2"/>`, e.Band, EclipseColour)
	}
	for _, run := range e.Centre {
		fmt.Fprintf(b,
			`<path d="%s" fill="none" stroke="%s" stroke-width="1.1" opacity="0.75"/>`,
			run, EclipseColour)
	}
	if e.Umbra == "" {
		return
	}

	// The umbra is a prop, not a clock: it loops in seconds where the real
	// shadow takes hours, so the label carries the date to keep that honest.
	fmt.Fprintf(b,
		`<circle class="umbra" r="4.5" fill="%s" style="offset-path:path('%s');animation-duration:%ds"/>`,
		EclipseColour, e.Umbra, e.Seconds)
}

// Station is the ISS. Leg is the stretch of ground track it is riding right
// now, used as a path to move along rather than drawn.
type Station struct {
	Leg     string
	Seconds int
	Delay   float64
}

func station(b *strings.Builder, s *Station) {
	if s == nil || s.Leg == "" {
		return
	}
	fmt.Fprintf(b,
		`<circle class="station" r="3.2" fill="%s" style="offset-path:path('%s');`+
			`animation-duration:%ds;animation-delay:%ss"/>`,
		ISS, s.Leg, s.Seconds, Num2(s.Delay))
	fmt.Fprintf(b,
		`<circle class="station" r="7" fill="%s" opacity="0.18" style="offset-path:path('%s');`+
			`animation-duration:%ds;animation-delay:%ss"/>`,
		ISS, s.Leg, s.Seconds, Num2(s.Delay))
}

// Ascent is the nominal track off the next pad: where the rocket would fly if
// everything went perfectly. A simulation, never a live position.
type Ascent struct {
	// Track is one path per crossing of the map's edge, and Ride is the run the
	// dot climbs once, set only while the rocket is actually airborne.
	Track   []string
	Ride    string
	Seconds int
}

// ascent draws quietly on purpose. It is a projection of a flight that has not
// happened, so it gets a thin line and no glow.
func ascent(b *strings.Builder, a *Ascent) {
	if a == nil {
		return
	}
	for _, run := range a.Track {
		fmt.Fprintf(b,
			`<path d="%s" fill="none" stroke="%s" stroke-width="0.9" opacity="0.45"`+
				` stroke-dasharray="2 3"/>`,
			run, LaunchNext)
	}
	if a.Ride == "" {
		return
	}
	fmt.Fprintf(b,
		`<circle class="ascent" r="2.6" fill="%s" style="offset-path:path('%s');`+
			`animation-duration:%ds"/>`,
		LaunchNext, a.Ride, a.Seconds)
}

// LaunchPad is a projected launch site. The soonest one gets the ring and the
// only label, so a crowded coast stays readable.
type LaunchPad struct {
	X, Y  float64
	Label string
	Next  bool
}

func launchPads(b *strings.Builder, pads []LaunchPad) {
	for _, p := range pads {
		if p.Next {
			continue
		}
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="2.6" fill="%s" opacity="0.85"/>`,
			Num(p.X), Num(p.Y), Launch)
	}
	for _, p := range pads {
		if p.Next {
			nextLaunch(b, p)
		}
	}
}

// labelSize has to be a constant the layout can read, not just a literal in the
// markup, because the label's width decides which side of the pad it goes on.
const labelSize = 11.5

// nextLaunch is the one pad worth looking at: a ring expanding out of it, and a
// label placed inboard so it never runs off the edge of the map.
func nextLaunch(b *strings.Builder, p LaunchPad) {
	fmt.Fprintf(b, `<g transform="translate(%s,%s)">`, Num(p.X), Num(p.Y))
	fmt.Fprintf(b, `<circle class="ping" r="4" fill="none" stroke="%s" stroke-width="1.4"/>`, LaunchNext)
	fmt.Fprintf(b, `<circle r="3.4" fill="%s"/>`, LaunchNext)
	b.WriteString(`</g>`)

	if p.Label == "" {
		return
	}
	// The label sits inside the map's clip path, so what decides the side is
	// where the text ends, not where the pad is.
	anchor, dx := "start", 9.0
	if p.X+dx+TextWidth(p.Label, labelSize) > MapW {
		anchor, dx = "end", -9.0
	}

	// Off the pad's own line, so an ascent arc leaving due east or west does not
	// run through the text. Below instead when the pad is too near the top edge.
	dy := -7.0
	if p.Y < labelSize+2 {
		dy = 15
	}
	fmt.Fprintf(b,
		`<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="%s">%s</text>`,
		Num(p.X+dx), Num(p.Y+dy), LaunchNext, Num(labelSize), anchor, Esc(p.Label))
}

func land(b *strings.Builder, path string) {
	fmt.Fprintf(b,
		`<path d="%s" fill="%s" stroke="%s" stroke-width="0.6" stroke-linejoin="round" fill-rule="evenodd"/>`,
		path, LandFill, LandStroke)
}

func legend(b *strings.Builder, items []LegendItem) {
	if len(items) == 0 {
		return
	}
	x := float64(Pad)
	for _, item := range items {
		fmt.Fprintf(b, `<circle cx="%s" cy="%s" r="4" fill="%s"/>`,
			Num(x+4), Num(LegendY-4), item.Colour)
		fmt.Fprintf(b, `<text x="%s" y="%s" fill="%s" font-size="11.5">%s</text>`,
			Num(x+14), Num(LegendY), Muted, Esc(item.Label))
		x += 14 + TextWidth(item.Label, 11.5) + 20
	}
}

// tickerSize, and the width a line has to live in. A launch line is the feed's
// mission name and site verbatim, so nothing upstream bounds how wide it gets.
const (
	tickerSize = 12.5
	tickerW    = MapW - 2*Pad
)

func ticker(b *strings.Builder, lines []string) {
	for i, line := range lines {
		if i >= MaxTickerRow {
			break
		}
		y := TickerY + float64(i)*TickerLineH
		fmt.Fprintf(b, `<text class="mono" x="%s" y="%s" fill="%s" font-size="%s">%s</text>`,
			Num(Pad), Num(y), Muted, Num(tickerSize), Esc(Ellipsis(line, tickerSize, tickerW)))
	}
}
