package render

import (
	"strings"
	"testing"
)

func draw(f func(*strings.Builder)) string {
	var b strings.Builder
	f(&b)
	return b.String()
}

func TestNextLaunchFlipsOnLabelWidth(t *testing.T) {
	const long = "30 Jul 06:12Z  Soyuz MS-29 crew rotation flight"

	cases := []struct {
		name   string
		x      float64
		label  string
		anchor string
	}{
		{"short label out west", 120, "30 Jul 06:12Z  Falcon 9", "start"},
		{"long label out west", 120, long, "start"},
		{"short label near the edge", 940, "30 Jul", "start"},
		// The pad sits past the old 0.72 threshold but its label still fits.
		{"baikonur, the one that nearly clipped", 676.6, long, "start"},
		{"long label near the edge", 780, long, "end"},
		{"pad on the edge itself", 999, "x", "end"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := draw(func(b *strings.Builder) {
				nextLaunch(b, LaunchPad{X: c.x, Y: 200, Label: c.label, Next: true})
			})
			if want := `text-anchor="` + c.anchor + `"`; !strings.Contains(got, want) {
				t.Fatalf("want %s:\n%s", want, got)
			}
			if c.anchor == "start" && c.x+9+TextWidth(c.label, labelSize) > MapW {
				t.Errorf("label runs off the map at x=%v", c.x)
			}
		})
	}
}

func TestNextLaunchWithoutLabel(t *testing.T) {
	got := draw(func(b *strings.Builder) {
		nextLaunch(b, LaunchPad{X: 400, Y: 200, Next: true})
	})
	if strings.Contains(got, "<text") {
		t.Errorf("drew an empty label:\n%s", got)
	}
}

func TestTickerTruncates(t *testing.T) {
	short := "30 Jul 06:12Z  Falcon 9  \u00b7  Cape Canaveral"
	long := "30 Jul 06:12Z  " + strings.Repeat("Long Duration Mission Name ", 12)

	got := draw(func(b *strings.Builder) { ticker(b, []string{short, long}) })

	if !strings.Contains(got, Esc(short)) {
		t.Errorf("a line that fits was changed:\n%s", got)
	}
	if strings.Contains(got, Esc(long)) {
		t.Errorf("an over-wide line was drawn in full:\n%s", got)
	}
	for _, text := range textContent(got) {
		if TextWidth(text, tickerSize) > tickerW {
			t.Errorf("ticker line is %.1f wide, over %.1f: %q", TextWidth(text, tickerSize), tickerW, text)
		}
	}
	if !strings.Contains(got, "\u2026</text>") {
		t.Errorf("truncated line has no ellipsis:\n%s", got)
	}
}

func TestTickerStopsAtMaxRows(t *testing.T) {
	lines := make([]string, MaxTickerRow+3)
	for i := range lines {
		lines[i] = "line"
	}
	if n := strings.Count(draw(func(b *strings.Builder) { ticker(b, lines) }), "<text"); n != MaxTickerRow {
		t.Errorf("drew %d rows, want %d", n, MaxTickerRow)
	}
}

func TestAuroraSkipsEmptyBands(t *testing.T) {
	got := draw(func(b *strings.Builder) {
		aurora(b, []Aurora{{Path: "", North: true}, {Path: "M0,0L1,1Z", North: false}})
	})
	if strings.Contains(got, `d=""`) {
		t.Errorf("drew an empty path:\n%s", got)
	}
	if n := strings.Count(got, "<path"); n != 1 {
		t.Errorf("drew %d paths, want 1:\n%s", n, got)
	}
}

// TestAscentDrawsTheArcWithoutADot covers a track split so hard by the
// antimeridian that no run is worth riding. The aim still draws; nothing moves.
func TestAscentDrawsTheArcWithoutADot(t *testing.T) {
	got := draw(func(b *strings.Builder) {
		ascent(b, &Ascent{Track: []string{"M0,0L40,20", "M960,0L1000,20"}, Seconds: 7})
	})
	if n := strings.Count(got, "<path"); n != 2 {
		t.Errorf("drew %d paths, want 2:\n%s", n, got)
	}
	if strings.Contains(got, "<circle") {
		t.Errorf("drew a dot with no run to ride:\n%s", got)
	}
}

func TestAscentSkipsAMissingLayer(t *testing.T) {
	if got := draw(func(b *strings.Builder) { ascent(b, nil) }); got != "" {
		t.Errorf("drew %q for no ascent", got)
	}
}

func TestTerminatorStrokesTheEdgePathWhenItHasOne(t *testing.T) {
	const night, edge = "M0,0L1000,0L1000,500L0,500Z", "M0,0L1000,0"

	// Without one, fill and stroke share the single closed path, as before.
	plain := draw(func(b *strings.Builder) {
		terminator(b, &Terminator{NightPath: night})
	})
	if n := strings.Count(plain, night); n != 2 {
		t.Errorf("night path drawn %d times, want once per copy:\n%s", n, plain)
	}
	if !strings.Contains(plain, `fill-opacity="0.55"`) || !strings.Contains(plain, `stroke-opacity="0.35"`) {
		t.Errorf("lost the fill or the stroke:\n%s", plain)
	}

	// With one, only the real boundary is stroked: no accent down the seam the
	// polygon closes along.
	got := draw(func(b *strings.Builder) {
		terminator(b, &Terminator{NightPath: night, EdgePath: edge})
	})
	if strings.Contains(got, `d="`+night+`" fill="none"`) {
		t.Errorf("still stroking the closed polygon:\n%s", got)
	}
	if n := strings.Count(got, `d="`+edge+`" fill="none"`); n != 2 {
		t.Errorf("stroked the edge path %d times, want once per copy:\n%s", n, got)
	}
}

func TestTerminatorSkipsEmpty(t *testing.T) {
	for _, term := range []*Terminator{nil, {}, {EdgePath: "M0,0L1,1"}} {
		if got := draw(func(b *strings.Builder) { terminator(b, term) }); got != "" {
			t.Errorf("drew %q for %+v", got, term)
		}
	}
}
