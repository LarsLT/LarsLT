package render

import (
	"fmt"
	"math/rand"
	"strings"
)

// LegendItem is one coloured chip under the map.
type LegendItem struct {
	Colour string
	Label  string
}

func background(b *strings.Builder) {
	fmt.Fprintf(b, `<rect width="%s" height="%s" fill="url(#sky)"/>`, Num(Width), Num(Height))
}

// starfield is seeded so the sky does not reshuffle on every rebuild.
func starfield(b *strings.Builder) {
	rng := rand.New(rand.NewSource(StarSeed))
	fmt.Fprintf(b, `<g fill="%s">`, Star)
	for range StarCount {
		x := rng.Float64() * Width
		y := rng.Float64() * Height
		bucket := rng.Float64()
		class, radius, opacity := "tw3", 0.4+rng.Float64()*0.3, "0.34"
		switch {
		case bucket > 0.86:
			class, radius, opacity = "tw1", 0.9+rng.Float64()*0.5, "0.9"
		case bucket > 0.5:
			class, radius, opacity = "tw2", 0.6+rng.Float64()*0.35, "0.6"
		}
		delay := -rng.Float64() * 8
		fmt.Fprintf(b,
			`<circle class="%s" cx="%s" cy="%s" r="%s" opacity="%s" style="animation-delay:%ss"/>`,
			class, Num(x), Num(y), Num2(radius), opacity, Num2(delay))
	}
	b.WriteString(`</g>`)
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

func header(b *strings.Builder, updated string) {
	fmt.Fprintf(b,
		`<text x="%s" y="28" fill="%s" font-size="17" font-weight="600" letter-spacing="1.6">%s</text>`,
		Num(Pad), Text, Esc(TitleText))
	fmt.Fprintf(b,
		`<text class="mono" x="%s" y="28" fill="%s" font-size="11.5" text-anchor="end">%s</text>`,
		Num(Width-Pad), Dim, Esc(updated))
	fmt.Fprintf(b, `<line x1="0" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1"/>`,
		Num(HeaderH-0.5), Num(Width), Num(HeaderH-0.5), Graticule)
}

func legend(b *strings.Builder, items []LegendItem) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, `<line x1="0" y1="%s" x2="%s" y2="%s" stroke="%s" stroke-width="1"/>`,
		Num(FooterY+0.5), Num(Width), Num(FooterY+0.5), Graticule)
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
