package render

import (
	"fmt"
	"strings"
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

// Aurora is one hemisphere's glow band, already projected and closed.
type Aurora struct {
	Path  string
	North bool
}

func aurora(b *strings.Builder, bands []Aurora) {
	for _, band := range bands {
		if band.Path == "" {
			continue
		}
		gradient := "auroraN"
		if !band.North {
			gradient = "auroraS"
		}
		fmt.Fprintf(b, `<path class="glow" d="%s" fill="url(#%s)"/>`, band.Path, gradient)
	}
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

// Station is the ISS: its ground track, and the leg of that track the station
// is riding right now.
type Station struct {
	Track   []string
	Leg     string
	Seconds int
	Delay   float64
}

func station(b *strings.Builder, s *Station) {
	if s == nil {
		return
	}
	for _, run := range s.Track {
		fmt.Fprintf(b,
			`<path d="%s" fill="none" stroke="%s" stroke-width="1" opacity="0.4" stroke-dasharray="4 3"/>`,
			run, ISS)
	}
	if s.Leg == "" {
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
	fmt.Fprintf(b,
		`<text x="%s" y="%s" fill="%s" font-size="%s" text-anchor="%s">%s</text>`,
		Num(p.X+dx), Num(p.Y+4), LaunchNext, Num(labelSize), anchor, Esc(p.Label))
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
