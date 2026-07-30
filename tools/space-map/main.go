// Command space-map renders the animated world map embedded in the profile
// README: launches, eclipses, aurora, terminator, ISS and meteor showers.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"math/rand"
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

// runBudget bounds every fetch put together. The workflow gives the job ten
// minutes, and a hung source must still leave time to draw the rest of the map.
const runBudget = 4 * time.Minute

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
	ctx, cancel := context.WithTimeout(context.Background(), runBudget)
	defer cancel()

	// The sun needs no network, so this layer is always present.
	sky.Terminator = buildTerminator(sky.Generated)
	sky.Legend = []render.LegendItem{
		{Colour: render.SunCore, Label: "daylight"},
	}

	addMeteors(&sky, now)
	addStation(ctx, client, &sky, now)
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

// A radiant is worth watching where it climbs well clear of the horizon, which
// is a broad band of latitude either side of its declination.
const (
	radiantReach = 70.0
	meteorSeed   = 1833 // the Leonid storm that started meteor science
	maxStreaks   = 34

	// Clear of the top edge by a long streak's rise, so none is drawn half cut.
	streakHeadroom = 6.0
)

// addMeteors scatters streaks over the latitudes the busiest running shower is
// seen from. Overlapping bands of dashes stop reading, so only one is drawn.
func addMeteors(sky *render.Sky, now time.Time) {
	shower, ok := data.StrongestShower(now)
	if !ok {
		log.Print("no meteor shower running")
		return
	}

	north := math.Min(90, shower.RadiantDec+radiantReach)
	south := math.Max(-90, shower.RadiantDec-radiantReach)

	// A streak is drawn up and to the left of the point it is scattered at, so
	// one landing flush against the top of the map has its head cut off.
	top, bottom := math.Max(geo.Y(north), streakHeadroom), geo.Y(south)

	// More meteors an hour, more streaks, but never a swarm.
	count := min(maxStreaks, 8+shower.ZHR/5)
	rng := rand.New(rand.NewSource(meteorSeed))
	for range count {
		sky.Meteors = append(sky.Meteors, render.Streak{
			X:     rng.Float64() * geo.MapW,
			Y:     top + rng.Float64()*(bottom-top),
			Scale: 0.7 + rng.Float64()*0.7,
			Delay: -rng.Float64() * 3.4,
		})
	}

	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.Meteor, Label: "meteor shower"})
	sky.Ticker = append(sky.Ticker, showerLine(shower, now))

	log.Printf("%s active, %d streaks between %.0f and %.0f latitude",
		shower.Name, count, south, north)
}

// showerLine says where the shower is in its run without pretending to count
// down, since a baked string cannot.
func showerLine(s data.Shower, now time.Time) string {
	peak := time.Date(now.Year(), s.Peak.Month, s.Peak.Day, 0, 0, 0, 0, time.UTC)
	switch days := s.PeakIn(now); {
	case days == 0:
		return fmt.Sprintf("%s peaking tonight  ·  up to %d an hour", s.Name, s.ZHR)
	case days > 0:
		return fmt.Sprintf("%s building  ·  peaks %s  ·  up to %d an hour",
			s.Name, peak.Format("02 Jan"), s.ZHR)
	default:
		return fmt.Sprintf("%s fading  ·  peaked %s  ·  up to %d an hour",
			s.Name, peak.Format("02 Jan"), s.ZHR)
	}
}

// trackStep is how finely the ground track is sampled. Half a minute is about
// four pixels of travel, so the curve stays smooth without bloating the file.
const trackStep = 30 * time.Second

// addStation draws the ISS ground track for an orbit either side of now, with
// the station itself riding the leg it is on.
func addStation(ctx context.Context, client *sources.Client, sky *render.Sky, now time.Time) {
	tle, err := sources.ISS(ctx, client, now)
	if err != nil {
		log.Printf("iss unavailable: %v", err)
		return
	}

	points, times := tle.GroundTrack(now, tle.Period(), trackStep)
	runs := trackRuns(points)

	st := &render.Station{}
	var leg [2]int
	for _, run := range runs {
		path := render.PathD(project(points[run[0]:run[1]+1]), false)
		if path == "" {
			continue
		}
		st.Track = append(st.Track, path)

		// The first leg not already behind us is the one the station is on, or,
		// in the step of time a split drops, the one it is a moment from joining.
		if st.Leg == "" && !times[run[1]].Before(now) {
			leg, st.Leg = run, path
		}
	}
	if st.Leg != "" {
		// The negative delay is what puts the station at today's position on load,
		// measured along the drawn line because that is what CSS walks.
		span := times[leg[1]].Sub(times[leg[0]])
		st.Seconds = int(span.Seconds())
		st.Delay = render.ArcPhaseDelay(points[leg[0]:leg[1]+1], times[leg[0]:leg[1]+1], now)

		// A leg still ahead of now would wrap the phase round to its far end, and
		// its start is where the station crosses the edge of the map anyway.
		if gap := times[leg[0]].Sub(now); gap > 0 {
			log.Printf("now falls %s short of the leg it rides, starting it from the edge", gap.Round(time.Second))
			st.Delay = 0
		}
	}

	sky.Station = st
	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.ISS, Label: "ISS ground track"})

	here := tle.SubPoint(now)
	log.Printf("ISS at %.1fN %.1fE, elements %s old, %d track legs",
		here.Lat, here.Lon, tle.Age(now).Round(time.Hour), len(st.Track))
}

// trackRuns splits a ground track wherever it leaves one edge of the map and
// comes back on the other, returning inclusive index ranges.
func trackRuns(points []geo.Point) [][2]int {
	var runs [][2]int
	start := 0
	for i := 1; i < len(points); i++ {
		if math.Abs(points[i].Lon-points[i-1].Lon) > 180 {
			runs = append(runs, [2]int{start, i - 1})
			start = i
		}
	}
	return append(runs, [2]int{start, len(points) - 1})
}

func project(points []geo.Point) []geo.XY {
	xy := make([]geo.XY, 0, len(points))
	for _, p := range points {
		xy = append(xy, geo.Project(p))
	}
	return xy
}

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
		Band:    render.PolygonPath(shadowBand(e)),
		Centre:  centre,
		Umbra:   longest(centre),
		Seconds: umbraLoop,
	}
	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.EclipseColour, Label: "eclipse path"})
	sky.Ticker = append(sky.Ticker, fmt.Sprintf("%s  ·  %s eclipse, greatest %s UT  ·  %s",
		when.Format("02 Jan 2006"), e.Kind, e.Greatest, e.Regions))

	log.Printf("next eclipse %s %s over %s", e.Date, e.Kind, e.Regions)
}

// longest is which run of a track the umbra is worth riding. A path split at
// the map's edge leaves the shadow crossing only the stretch it spends longest.
func longest(runs []string) string {
	best := ""
	for _, run := range runs {
		if len(run) > len(best) {
			best = run
		}
	}
	return best
}

// maxLimitSpread is how far apart the shadow's edges may sit in longitude
// before the band is dropped, since near a pole this projection smears it.
const maxLimitSpread = 25.0

// shadowBand closes the two limits into the strip the shadow sweeps, over the
// longest stretch where the projection still tells the truth about its width.
func shadowBand(e *data.Eclipse) []geo.Point {
	// The two limits are traced in step. If the table ever disagrees, band the
	// stretch they share rather than letting one layer take the build down.
	traced := min(len(e.North), len(e.South))
	if traced != max(len(e.North), len(e.South)) {
		log.Printf("eclipse %s has %d north and %d south points", e.Date, len(e.North), len(e.South))
	}

	best, current := [2]int{0, 0}, -1
	for i := range traced {
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

	// A ring is filled as drawn, so one that leaves the map at an edge and comes
	// back at the other is a stripe across the world rather than a shadow.
	for i := 1; i < len(band); i++ {
		if math.Abs(band[i].Lon-band[i-1].Lon) > 180 {
			log.Printf("eclipse %s crosses the map's edge, drawing the centre line only", e.Date)
			return nil
		}
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
			Path:  render.PolygonPath(ring),
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
		render.LegendItem{Colour: render.Launch, Label: "launch pad, next 30 days"},
	)

	log.Printf("%d launches from %d pads, next %s", len(launches), len(pads), launches[0].At.Format(time.RFC3339))
}

// tickerLaunches is how many flights fit under the map without crowding it, now
// that every other layer wants a line too.
const tickerLaunches = 1

func launchLabel(l sources.Launch) string {
	return fmt.Sprintf("%s  %s", launchWhen(l), l.Name)
}

// tickerLine spells the T-0 out in full. A countdown would be a lie the moment
// the file is cached, so the SVG only ever states absolute times.
func tickerLine(l sources.Launch) string {
	site := l.Site
	if site != "" {
		site = "  ·  " + site
	}
	return fmt.Sprintf("%s  %s%s", launchWhen(l), l.Name, site)
}

// launchWhen never writes a clock the feed did not give: a day-precision T-0
// says the day and admits the rest.
func launchWhen(l sources.Launch) string {
	if l.Vague {
		return l.At.Format("02 Jan") + " (TBD)"
	}
	return l.At.Format("02 Jan 15:04Z")
}

func buildTerminator(now time.Time) *render.Terminator {
	subsolar := astro.SubsolarPoint(now)
	night := astro.NightPolygon(subsolar, terminatorStepDeg)
	sun := geo.Project(subsolar)

	log.Printf("subsolar point %.2fN %.2fE", subsolar.Lat, subsolar.Lon)

	return &render.Terminator{
		NightPath: render.PolygonPath(night),
		EdgePath:  render.PathD(project(astro.TerminatorTrace(subsolar, terminatorStepDeg)), false),
		SunX:      sun.X,
		SunY:      sun.Y,
	}
}
