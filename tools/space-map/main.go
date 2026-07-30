// Command space-map renders the animated world map embedded in the profile
// README: launches, eclipses, aurora, terminator, ISS and meteor showers.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/data"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/astro"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/render"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/sources"
)

func main() {
	out := flag.String("out", "dist", "directory to write space-map.svg into")
	offline := flag.Bool("offline", false, "force every network source to fail, for testing degradation")
	at := flag.String("at", "", "render the sky at an RFC3339 instant instead of now, for eyeballing other times of day")
	cache := flag.String("cache", "cache", "directory holding the last good response from each source")
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("space-map: ")

	now := time.Now().UTC()
	if *at != "" {
		parsed, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			log.Fatalf("parse -at: %v", err)
		}
		now = parsed.UTC()
	}

	if err := run(*out, *cache, *offline, now); err != nil {
		log.Fatal(err)
	}
}

func run(outDir, cacheDir string, offline bool, now time.Time) error {
	sky := render.Sky{Generated: now}

	// The basemap is embedded, so this only fails if the binary is broken.
	bm, err := data.LoadBasemap()
	if err != nil {
		return err
	}
	sky.LandPath = bm.Land

	if offline {
		log.Print("offline mode, no sources fetched")
	}
	client := sources.New(cacheDir, offline)
	ctx := context.Background()

	// The sun needs no network, so this layer is always present.
	sky.Terminator = buildTerminator(sky.Generated)
	sky.Legend = []render.LegendItem{
		{Colour: render.SunCore, Label: "daylight"},
	}

	addEclipse(&sky, now)
	addAurora(ctx, client, &sky, now)
	addLaunches(ctx, client, &sky, now)

	svg := render.Document(sky)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	path := filepath.Join(outDir, "space-map.svg")
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return fmt.Errorf("write svg: %w", err)
	}

	log.Printf("wrote %s (%.1f KB)", path, float64(len(svg))/1024)
	return nil
}

// terminatorStepDeg is how finely the day/night boundary is traced. Two degrees
// is under three pixels of spacing on a 1000px map.
const terminatorStepDeg = 2.0

// umbraLoop is how long the shadow takes to cross the map. The real crossing
// runs to hours, so this is openly a loop rather than a clock.
const umbraLoop = 14

// addEclipse draws the next central solar eclipse: the shadow's band, the
// centre line, and an umbra running along it.
func addEclipse(sky *render.Sky, now time.Time) {
	e, err := data.NextEclipse(now)
	if err != nil {
		log.Printf("eclipse unavailable: %v", err)
		return
	}
	when, _ := e.When()

	centre := render.TrackD(e.Central)
	if len(centre) == 0 {
		log.Print("eclipse has no drawable centre line")
		return
	}

	sky.Eclipse = &render.Eclipse{
		Band:    render.PolygonPath(shadowBand(e), 0),
		Centre:  centre[0],
		Seconds: umbraLoop,
	}
	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.EclipseColour, Label: "eclipse path"})
	sky.Ticker = append(sky.Ticker, fmt.Sprintf("%s  ·  %s eclipse, greatest %s UT  ·  %s",
		when.Format("02 Jan 2006"), e.Kind, e.Greatest, e.Regions))

	log.Printf("next eclipse %s %s over %s", e.Date, e.Kind, e.Regions)
}

// maxLimitSpread is how far apart the shadow's edges may sit in longitude
// before the band is dropped, since near a pole this projection smears it.
const maxLimitSpread = 25.0

// shadowBand closes the two limits into the strip the shadow sweeps, over the
// longest stretch where the projection still tells the truth about its width.
func shadowBand(e *data.Eclipse) []geo.Point {
	best, current := [2]int{0, 0}, -1
	for i := range e.North {
		spread := math.Abs(geo.WrapLon(e.North[i].Lon - e.South[i].Lon))
		if spread > maxLimitSpread {
			current = -1
			continue
		}
		if current < 0 {
			current = i
		}
		if i-current > best[1]-best[0] {
			best = [2]int{current, i}
		}
	}
	if best[1]-best[0] < 2 {
		return nil
	}

	north, south := e.North[best[0]:best[1]+1], e.South[best[0]:best[1]+1]
	band := make([]geo.Point, 0, len(north)+len(south))
	band = append(band, north...)
	for i := len(south) - 1; i >= 0; i-- {
		band = append(band, south[i])
	}
	return band
}

// ovalStepDeg traces the auroral edges at the same resolution as the sun's.
const ovalStepDeg = 2.0

// homeLon is the meridian the aurora sentence is written for. The map is on a
// Dutch profile, so "how far south" means how far south over the Netherlands.
const homeLon = 5.0

// addAurora draws both ovals from the current Kp and says in plain words how
// far south the glow reaches.
func addAurora(ctx context.Context, client *sources.Client, sky *render.Sky, now time.Time) {
	kp, err := sources.Kp(ctx, client, now)
	if err != nil {
		log.Printf("aurora unavailable: %v", err)
		return
	}

	for _, north := range []bool{true, false} {
		ring := astro.Oval(kp.Kp, north, ovalStepDeg)
		sky.Aurora = append(sky.Aurora, render.Aurora{
			Path:  render.PolygonPath(ring, 0),
			North: north,
		})
	}

	reach := astro.GeographicLatAt(astro.VisibleFrom(kp.Kp), homeLon, true)
	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.AuroraCore, Label: "auroral oval"})
	sky.Ticker = append(sky.Ticker, auroraLine(kp.Kp, reach))

	log.Printf("Kp %.2f at %s, visible down to %.1fN over the Netherlands",
		kp.Kp, kp.At.Format(time.RFC3339), reach)
}

// auroraLine turns a Kp number into the thing people actually want to know.
func auroraLine(kp, reach float64) string {
	where := fmt.Sprintf("visible down to %.0fN", reach)
	if reach <= dutchLat {
		where = "visible from the Netherlands"
	}
	return fmt.Sprintf("Kp %.1f  ·  aurora %s", kp, where)
}

// dutchLat is roughly the middle of the country, the line the sentence flips on.
const dutchLat = 52.5

// addLaunches puts a dot on every pad flying in the next 30 days, rings the
// soonest one, and lists the next few under the map.
func addLaunches(ctx context.Context, client *sources.Client, sky *render.Sky, now time.Time) {
	launches, err := sources.Launches(ctx, client, now)
	if err != nil {
		log.Printf("launches unavailable: %v", err)
		return
	}
	if len(launches) == 0 {
		log.Print("no launches in the window")
		return
	}

	// Launches come sorted, and Pads keeps that order, so the first pad is the
	// one flying next.
	pads := sources.Pads(launches)
	for i, pad := range pads {
		xy := geo.Project(pad.Position)
		label := ""
		if i == 0 {
			label = launchLabel(pad.Next)
		}
		sky.Launches = append(sky.Launches, render.LaunchPad{
			X: xy.X, Y: xy.Y, Label: label, Next: i == 0,
		})
	}

	for _, l := range launches[:min(len(launches), tickerLaunches)] {
		sky.Ticker = append(sky.Ticker, tickerLine(l))
	}
	sky.Legend = append(sky.Legend,
		render.LegendItem{Colour: render.LaunchNext, Label: "next launch"},
		render.LegendItem{Colour: render.Launch, Label: "pad flying within 30 days"},
	)

	log.Printf("%d launches from %d pads, next %s", len(launches), len(pads), launches[0].At.Format(time.RFC3339))
}

// tickerLaunches is how many flights fit under the map without crowding it.
const tickerLaunches = 2

func launchLabel(l sources.Launch) string {
	return fmt.Sprintf("%s  %s", l.At.Format("02 Jan 15:04Z"), l.Name)
}

// tickerLine spells the T-0 out in full. A countdown would be a lie the moment
// the file is cached, so the SVG only ever states absolute times.
func tickerLine(l sources.Launch) string {
	when := l.At.Format("02 Jan 15:04Z")
	if l.Vague {
		when = l.At.Format("02 Jan") + " (TBD)"
	}
	site := l.Site
	if site != "" {
		site = "  ·  " + site
	}
	return fmt.Sprintf("%s  %s%s", when, l.Name, site)
}

func buildTerminator(now time.Time) *render.Terminator {
	subsolar := astro.SubsolarPoint(now)
	night := astro.NightPolygon(subsolar, terminatorStepDeg)
	sun := geo.Project(subsolar)

	log.Printf("subsolar point %.2fN %.2fE", subsolar.Lat, subsolar.Lon)

	return &render.Terminator{
		NightPath: render.PolygonPath(night, 0),
		SunX:      sun.X,
		SunY:      sun.Y,
	}
}
