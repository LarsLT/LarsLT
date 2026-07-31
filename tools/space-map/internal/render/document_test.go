package render

import (
	"encoding/xml"
	"io"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// xmlErr reports whether a fragment parses, so a test can assert both ways.
func xmlErr(doc string) error {
	dec := xml.NewDecoder(strings.NewReader(doc))
	for {
		if _, err := dec.Token(); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func parseXML(t *testing.T, doc string) {
	t.Helper()
	if err := xmlErr(doc); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
}

// textContent is every <text> element's body, which is what the map actually
// shows and the only place feed strings end up.
func textContent(doc string) []string {
	var out []string
	dec := xml.NewDecoder(strings.NewReader(doc))
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		if s, ok := tok.(xml.StartElement); ok && s.Name.Local == "text" {
			var body string
			if err := dec.DecodeElement(&body, &s); err == nil {
				out = append(out, body)
			}
		}
	}
}

var (
	stripped = regexp.MustCompile(`(?i)<script|<animate|<set |onload=|javascript:`)
	external = regexp.MustCompile(`(?:href|src|xlink:href)="https?://[^"]+`)
)

// checkDocument is the workflow's gate as a unit test: anything GitHub strips
// or camo blocks is a broken image on the profile, found half an hour late.
func checkDocument(t *testing.T, doc string) {
	t.Helper()
	parseXML(t, doc)
	if m := stripped.FindString(doc); m != "" {
		t.Errorf("GitHub strips %q", m)
	}
	for _, ref := range external.FindAllString(doc, -1) {
		if !strings.Contains(ref, "w3.org/") {
			t.Errorf("external reference: %s", ref)
		}
	}
	if strings.Contains(doc, "NaN") || strings.Contains(doc, "Inf") {
		t.Errorf("non-finite number reached the output")
	}
}

func TestDocumentZeroSky(t *testing.T) {
	// Every source may fail at once, and the map still has to render.
	doc := Document(Sky{})
	checkDocument(t, doc)
	if !strings.HasPrefix(doc, `<svg xmlns="http://www.w3.org/2000/svg"`) || !strings.HasSuffix(doc, "</svg>") {
		t.Error("not a self-contained svg element")
	}
	if strings.Contains(doc, `d=""`) {
		t.Errorf("drew an empty path")
	}
}

func TestDocumentFull(t *testing.T) {
	ring := PolygonPath([]geo.Point{{Lon: -180, Lat: 60}, {Lon: 0, Lat: 70}, {Lon: 180, Lat: 60}})
	doc := Document(Sky{
		LandPath:   "M0,0L10,10Z",
		Terminator: &Terminator{NightPath: ring, EdgePath: "M0,0L1000,250", SunX: 500, SunY: 250},
		Aurora:     []Aurora{{Path: ring, North: true}, {Path: ring}},
		Eclipse:    &Eclipse{Band: ring, Centre: []string{"M0,0L400,500", "M600,0L1000,500"}, Umbra: "M0,0L400,500", Seconds: 14},
		Station:    &Station{Track: []string{"M0,0L500,250"}, Leg: "M0,0L500,250", Seconds: 2700, Delay: -1234.5},
		Ascent:     &Ascent{Track: []string{"M0,0L120,90"}, Ride: "M0,0L120,90", Seconds: 7},
		Launches: []LaunchPad{
			{X: 200, Y: 300},
			{X: 780, Y: 120, Label: "30 Jul 06:12Z  Soyuz MS-29 crew rotation flight", Next: true},
		},
		Legend: []LegendItem{{Colour: SunCore, Label: "daylight"}},
		Ticker: []string{"Kp 4.3  ·  aurora visible down to 58N"},
	})
	checkDocument(t, doc)
}

// TestDocumentSurvivesHostileFeedText is the whole point of the escaping: a
// mission name is upstream text, and it lands in three different places.
func TestDocumentSurvivesHostileFeedText(t *testing.T) {
	doc := Document(Sky{
		Launches: []LaunchPad{{X: 400, Y: 200, Label: hostile, Next: true}},
		Legend:   []LegendItem{{Colour: Launch, Label: hostile}},
		Ticker:   []string{hostile, hostile},
	})
	checkDocument(t, doc)
	for _, text := range textContent(doc) {
		if strings.ContainsAny(text, "\v\x00") {
			t.Errorf("control byte reached the output: %q", text)
		}
	}
}

// TestDocumentSurvivesNonFiniteGeometry guards the whole file against one bad
// number: a NaN coordinate would otherwise take every layer down with it.
func TestDocumentSurvivesNonFiniteGeometry(t *testing.T) {
	nan, inf := math.NaN(), math.Inf(1)
	checkDocument(t, Document(Sky{
		Terminator: &Terminator{NightPath: PathD([]geo.XY{{X: nan, Y: 0}, {X: 10, Y: inf}}, true), SunX: nan, SunY: inf},
		Launches:   []LaunchPad{{X: nan, Y: inf, Label: "somewhere", Next: true}},
		Station:    &Station{Leg: "M0,0L1,1", Seconds: 1, Delay: nan},
	}))
}
