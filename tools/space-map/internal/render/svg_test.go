package render

import (
	"math"
	"strings"
	"testing"
)

// hostile is the kind of string a mission name can be: every character XML
// cares about, non-ASCII that must survive, and a control byte that must not.
const hostile = "Falcon 9 & \"Heavy\" <b>Starlink</b> 'A' \u2014 \u00fcn\u00efc\u00f8de\v\x00"

func TestEscEscapesMarkupAndKeepsUnicode(t *testing.T) {
	got := Esc(hostile)

	for _, want := range []string{"&amp;", "&quot;", "&lt;b&gt;", "&apos;", "\u2014", "\u00fcn\u00efc\u00f8de"} {
		if !strings.Contains(got, want) {
			t.Errorf("Esc(hostile) missing %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{"\v", "\x00", "<b>", `"`} {
		if strings.Contains(got, bad) {
			t.Errorf("Esc(hostile) still contains %q:\n%s", bad, got)
		}
	}
}

func TestEscMakesTextParsable(t *testing.T) {
	// The raw string is what breaks the build, so prove the guard is the thing
	// that fixes it rather than trusting the escaper alone.
	if err := xmlErr(`<t a="x">` + strings.ReplaceAll(hostile, "&", "&amp;") + `</t>`); err == nil {
		t.Fatal("control bytes parsed as XML, so this test proves nothing")
	}
	parseXML(t, `<t a="`+Esc(hostile)+`">`+Esc(hostile)+`</t>`)
}

func TestEscLeavesCleanTextAlone(t *testing.T) {
	const clean = "Starlink Group 12-5  \u00b7  Cape Canaveral"
	if got := Esc(clean); got != clean {
		t.Errorf("Esc(%q) = %q", clean, got)
	}
}

func TestNumHandlesNonFinite(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := Num(v); got != "0" {
			t.Errorf("Num(%v) = %q, want 0", v, got)
		}
		if got := Num2(v); got != "0" {
			t.Errorf("Num2(%v) = %q, want 0", v, got)
		}
	}
}

func TestNum(t *testing.T) {
	cases := []struct {
		in       float64
		one, two string
	}{
		{0, "0", "0"},
		{math.Copysign(0, -1), "0", "0"},
		{-0.004, "0", "0"},
		{1e-300, "0", "0"},
		{-1e-300, "0", "0"},
		{0.04, "0", "0.04"},
		{-0.04, "0", "-0.04"},
		{203.75, "203.8", "203.75"},
		{-676.6, "-676.6", "-676.6"},
		{1e21, "1000000000000000000000", "1000000000000000000000"},
	}
	for _, c := range cases {
		if got := Num(c.in); got != c.one {
			t.Errorf("Num(%v) = %q, want %q", c.in, got, c.one)
		}
		if got := Num2(c.in); got != c.two {
			t.Errorf("Num2(%v) = %q, want %q", c.in, got, c.two)
		}
	}
}

func TestNumNeverWritesAnExponent(t *testing.T) {
	// SVG path data has no exponent notation, so a coordinate that formats as
	// 1e-07 is a shape that silently vanishes.
	for e := -320; e <= 300; e++ {
		v := math.Pow(10, float64(e))
		for _, got := range []string{Num(v), Num(-v), Num2(v), Num2(-v)} {
			if strings.ContainsAny(got, "eE") {
				t.Fatalf("10^%d formatted as %q", e, got)
			}
		}
	}
}

func TestEllipsis(t *testing.T) {
	const size, width = 12.5, 968.0

	short := "26 Jul 09:04Z  Starlink Group 12-5"
	if got := Ellipsis(short, size, width); got != short {
		t.Errorf("Ellipsis trimmed a line that fits: %q", got)
	}

	long := strings.Repeat("Baikonur Cosmodrome Site 31/6 ", 20)
	got := Ellipsis(long, size, width)
	if !strings.HasSuffix(got, "\u2026") {
		t.Errorf("Ellipsis(long) = %q, want an ellipsis", got)
	}
	if TextWidth(got, size) > width {
		t.Errorf("Ellipsis(long) is %.1f wide, over %.1f", TextWidth(got, size), width)
	}
	if !strings.HasPrefix(long, strings.TrimSuffix(got, "\u2026")) {
		t.Errorf("Ellipsis(long) is not a prefix of the input: %q", got)
	}

	if got := Ellipsis("wide", size, 1); got != "\u2026" {
		t.Errorf("Ellipsis with no room = %q, want just the ellipsis", got)
	}
}
