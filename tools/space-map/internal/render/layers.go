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
