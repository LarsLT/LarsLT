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
	SunX      float64
	SunY      float64
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
		fmt.Fprintf(b, `<path d="%s" fill="%s" opacity="0.55"/>`, term.NightPath, Night)
		fmt.Fprintf(b,
			`<path d="%s" fill="none" stroke="%s" stroke-width="0.8" opacity="0.35"/>`,
			term.NightPath, Accent)
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
	Band    string
	Centre  string
	Seconds int
}

func eclipse(b *strings.Builder, e *Eclipse) {
	if e == nil || e.Centre == "" {
		return
	}
	if e.Band != "" {
		fmt.Fprintf(b, `<path d="%s" fill="%s" opacity="0.2"/>`, e.Band, EclipseColour)
	}
	fmt.Fprintf(b,
		`<path id="umbratrack" d="%s" fill="none" stroke="%s" stroke-width="1.1" opacity="0.75"/>`,
		e.Centre, EclipseColour)

	// The umbra is a prop, not a clock: it loops in seconds where the real
	// shadow takes hours, so the label carries the date to keep that honest.
	fmt.Fprintf(b,
		`<circle class="umbra" r="4.5" fill="%s" style="offset-path:path('%s');animation-duration:%ds"/>`,
		EclipseColour, e.Centre, e.Seconds)
}

// Streak is one meteor, drawn as a short diagonal dash that fades as it falls.
type Streak struct {
	X, Y  float64
	Scale float64
	Delay float64
}

func meteors(b *strings.Builder, streaks []Streak) {
	if len(streaks) == 0 {
		return
	}
	fmt.Fprintf(b, `<g stroke="%s" stroke-width="1.1" stroke-linecap="round">`, Meteor)
	for _, s := range streaks {
		length := 7 * s.Scale
		fmt.Fprintf(b,
			`<line class="streak" x1="%s" y1="%s" x2="%s" y2="%s" style="animation-delay:%ss"/>`,
			Num(s.X), Num(s.Y), Num(s.X-length), Num(s.Y-length*0.55), Num2(s.Delay))
	}
	b.WriteString(`</g>`)
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
	anchor, dx := "start", 9.0
	if p.X > MapW*0.72 {
		anchor, dx = "end", -9.0
	}
	fmt.Fprintf(b,
		`<text x="%s" y="%s" fill="%s" font-size="11.5" text-anchor="%s">%s</text>`,
		Num(p.X+dx), Num(p.Y+4), LaunchNext, anchor, Esc(p.Label))
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

func ticker(b *strings.Builder, lines []string) {
	for i, line := range lines {
		if i >= MaxTickerRow {
			break
		}
		y := TickerY + float64(i)*TickerLineH
		fmt.Fprintf(b, `<text class="mono" x="%s" y="%s" fill="%s" font-size="12.5">%s</text>`,
			Num(Pad), Num(y), Muted, Esc(line))
	}
}
