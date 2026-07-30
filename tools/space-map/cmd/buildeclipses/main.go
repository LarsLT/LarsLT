// Command buildeclipses turns Fred Espenak's path tables at eclipse.gsfc.nasa.gov
// into data/eclipses.json. Run once, the geometry is fixed centuries ahead.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// pathURL is where each eclipse's 120-second table lives.
const pathURL = "https://eclipse.gsfc.nasa.gov/SEpath/SEpath2001/%spath.html"

// catalogue is every solar eclipse with a central path between 2026 and 2030.
// 2029 has four partials and no path at all, so it is absent on purpose.
var catalogue = []struct {
	Code    string
	Date    string
	Kind    string
	Regions string
}{
	{"SE2026Feb17A", "2026-02-17", "annular", "Antarctica"},
	{"SE2026Aug12T", "2026-08-12", "total", "Greenland, Iceland and northern Spain"},
	{"SE2027Feb06A", "2027-02-06", "annular", "Chile and Argentina"},
	{"SE2027Aug02T", "2027-08-02", "total", "Spain, Morocco, Egypt and Saudi Arabia"},
	{"SE2028Jan26A", "2028-01-26", "annular", "Ecuador, Peru, Brazil and Iberia"},
	{"SE2028Jul22T", "2028-07-22", "total", "Australia and New Zealand"},
	{"SE2030Jun01A", "2030-06-01", "annular", "Algeria, Greece, Russia and Japan"},
	{"SE2030Nov25T", "2030-11-25", "total", "southern Africa and Australia"},
}

// Eclipse is one central eclipse, ready to draw.
type Eclipse struct {
	Date     string  `json:"date"`
	Kind     string  `json:"kind"`
	Regions  string  `json:"regions"`
	Greatest string  `json:"greatest"`
	North    []Point `json:"north"`
	South    []Point `json:"south"`
	Central  []Point `json:"central"`
}

// Point is a lon/lat pair, rounded to a tenth of a degree like the basemap.
type Point struct {
	Lon float64 `json:"lon"`
	Lat float64 `json:"lat"`
}

func main() {
	out := flag.String("out", "data/eclipses.json", "file to write")
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("buildeclipses: ")

	var eclipses []Eclipse
	for _, entry := range catalogue {
		body, err := fetch(fmt.Sprintf(pathURL, entry.Code))
		if err != nil {
			log.Fatalf("%s: %v", entry.Code, err)
		}
		e, err := parse(body)
		if err != nil {
			log.Fatalf("%s: %v", entry.Code, err)
		}
		e.Date, e.Kind, e.Regions = entry.Date, entry.Kind, entry.Regions
		eclipses = append(eclipses, e)
		log.Printf("%s %-7s %3d central points, greatest %s UT",
			entry.Date, entry.Kind, len(e.Central), e.Greatest)
	}

	blob, err := json.Marshal(eclipses)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, blob, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s (%.1f KB)", *out, float64(len(blob))/1024)
}

func fetch(url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

var (
	tagRE = regexp.MustCompile(`<[^>]+>`)
	// A row is a UT time, then three lat/lon pairs that may each be a dash.
	rowRE = regexp.MustCompile(`^\s*(\d\d):(\d\d)\s+(.+?)\s+\d\.\d{3}\s`)
	// Coordinates read "75 56.2N" or "108 45.5E", degrees and decimal minutes.
	coordRE = regexp.MustCompile(`(\d+)\s+(\d+\.\d+)([NSEW])`)
)

// parse pulls the three limit tracks out of one path table. The "Limits" rows
// bracketing it are the path's extremities rather than moments, and are skipped.
func parse(html string) (Eclipse, error) {
	text := tagRE.ReplaceAllString(html, "")

	var e Eclipse
	var times []string
	for line := range strings.SplitSeq(text, "\n") {
		m := rowRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		coords := coordRE.FindAllStringSubmatch(m[3], -1)

		// A full row carries all three tracks. Where a limit has run off the
		// Earth the row is short, and gets dropped whole.
		if len(coords) != 6 {
			continue
		}
		e.North = append(e.North, point(coords[0], coords[1]))
		e.South = append(e.South, point(coords[2], coords[3]))
		e.Central = append(e.Central, point(coords[4], coords[5]))
		times = append(times, m[1]+":"+m[2])
	}

	if len(e.Central) < 2 {
		return e, fmt.Errorf("found %d usable rows", len(e.Central))
	}
	// Greatest eclipse is the middle of the traced path, near enough for a label.
	e.Greatest = times[len(times)/2]
	return e, nil
}

func point(lat, lon []string) Point {
	return Point{Lon: round(degrees(lon)), Lat: round(degrees(lat))}
}

func degrees(m []string) float64 {
	deg, _ := strconv.ParseFloat(m[1], 64)
	minutes, _ := strconv.ParseFloat(m[2], 64)
	value := deg + minutes/60
	if m[3] == "S" || m[3] == "W" {
		value = -value
	}
	return value
}

func round(v float64) float64 { return math.Round(v*10) / 10 }
