// Command space-map renders the animated world map embedded in the profile
// README: launches, eclipses, aurora, the terminator and the ISS.
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

// trackStep is how finely the ground track is sampled. Half a minute is about
// four pixels of travel, so the curve stays smooth without bloating the file.
const trackStep = 30 * time.Second

// addStation draws half an orbit of ground track either side of now, with the
// station riding the leg it is on. A whole orbit each way reads as clutter.
func addStation(ctx context.Context, client *sources.Client, sky *render.Sky, now time.Time) {
	tle, err := sources.ISS(ctx, client, now)
	if err != nil {
		log.Printf("iss unavailable: %v", err)
		return
	}

	points, times := tle.GroundTrack(now, tle.Period()/2, trackStep)
	runs := trackRuns(points)

	st := &render.Station{}
	var leg [2]int
	for _, run := range runs {
		path := render.PathD(project(points[run[0]:run[1]+1]), false)
		if path == "" {
			continue
		}
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
	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.ISS, Label: "ISS"})

	here := tle.SubPoint(now)
	log.Printf("ISS at %.1fN %.1fE, elements %s old, %d track legs",
		here.Lat, here.Lon, tle.Age(now).Round(time.Hour), len(runs))
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

// eclipseLead is how long before an eclipse its path is worth drawing. Months
// of notice is not news, and every other layer on the map is about today.
const eclipseLead = 48 * time.Hour

// tableStep is the spacing the paths are traced at, which is the only thing
// that dates a point: the count of them is how long the shadow is on the Earth.
const tableStep = 120 * time.Second

// addEclipse draws the next central solar eclipse once it is near: the shadow's
// band, the centre line, and, while it is happening, an umbra running along it.
func addEclipse(sky *render.Sky, now time.Time) {
	e, err := data.NextEclipse(now)
	if err != nil {
		log.Printf("eclipse unavailable: %v", err)
		return
	}
	when, _ := e.When()
	if lead := when.Sub(now); lead > eclipseLead {
		log.Printf("next eclipse is %s off, too far to draw", lead.Round(time.Hour))
		return
	}

	lo, hi, ok := drawableRun(e)
	if !ok {
		log.Print("eclipse has no stretch this projection can draw")
		return
	}
	centre := render.TrackD(e.Central[lo : hi+1])
	if len(centre) == 0 {
		log.Print("eclipse has no drawable centre line")
		return
	}

	sky.Eclipse = &render.Eclipse{
		Band:    render.PolygonPath(shadowBand(e, lo, hi)),
		Centre:  centre,
		Seconds: umbraLoop,
	}

	// The shadow only rides the path while it is actually on the Earth. Outside
	// those couple of hours the line is a forecast, and nothing should move.
	half := time.Duration(len(e.Central)-1) * tableStep / 2
	if now.After(when.Add(-half)) && now.Before(when.Add(half)) {
		sky.Eclipse.Umbra = longest(centre)
		log.Printf("eclipse under way, greatest %s UT", e.Greatest)
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

// drawableRun is the longest stretch of the traced path this projection can
// still tell the truth about. Towards a pole it smears the shadow sideways.
func drawableRun(e *data.Eclipse) (int, int, bool) {
	// The three paths are traced in step. If the table ever disagrees, use the
	// stretch they share rather than letting one layer take the build down.
	traced := min(len(e.North), len(e.South), len(e.Central))
	if traced != max(len(e.North), len(e.South), len(e.Central)) {
		log.Printf("eclipse %s traces %d north, %d south, %d central points",
			e.Date, len(e.North), len(e.South), len(e.Central))
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
		return 0, 0, false
	}
	return best[0], best[1], true
}

// shadowBand closes the two limits into the strip the shadow sweeps.
func shadowBand(e *data.Eclipse, lo, hi int) []geo.Point {
	north, south := e.North[lo:hi+1], e.South[lo:hi+1]
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

	// The sentence describes the band that is drawn, not a wider estimate of who
	// could glimpse it, so the words and the picture cannot disagree.
	edge := astro.GeographicLatAt(astro.OvalBoundary(kp.Kp), homeLon, true)
	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.AuroraCore, Label: "auroral oval"})
	sky.Ticker = append(sky.Ticker, auroraLine(kp.Kp, edge))

	log.Printf("Kp %.2f at %s, oval reaches %.1fN over the Netherlands",
		kp.Kp, kp.At.Format(time.RFC3339), edge)
}

// auroraLine turns a Kp number into the thing people actually want to know.
func auroraLine(kp, edge float64) string {
	where := fmt.Sprintf("reaches %.0fN", edge)
	if edge <= dutchLat {
		where = "overhead in the Netherlands"
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

	addAscent(sky, pads[0].Next)

	for _, l := range launches[:min(len(launches), tickerLaunches)] {
		sky.Ticker = append(sky.Ticker, tickerLine(l))
	}
	sky.Legend = append(sky.Legend,
		render.LegendItem{Colour: render.LaunchNext, Label: "next launch, nominal ascent"},
		render.LegendItem{Colour: render.Launch, Label: "launch pad, next 30 days"},
	)

	log.Printf("%d launches from %d pads, next %s", len(launches), len(pads), launches[0].At.Format(time.RFC3339))
}

// tickerLaunches is how many flights fit under the map without crowding it, now
// that every other layer wants a line too.
const tickerLaunches = 1

// The ascent arc: roughly the ground distance a rocket covers before insertion,
// sampled fine enough to stay smooth, and looped in seconds.
const (
	ascentArcDeg  = 25
	ascentStepDeg = 1
	ascentLoop    = 7
)

// nominalInclination is the aim inferred from the orbit class, since the feed
// carries no inclination field. Anything else flies the minimum-energy case.
var nominalInclination = map[string]float64{
	"SSO": 97.8,
	"PO":  90,
	"MEO": 55,
}

// noAscent are the missions an insertion arc says nothing about: a hop that
// never reaches orbit, and the trajectories that leave it.
var noAscent = map[string]bool{"Sub": true, "Mars": true, "L2": true}

// addAscent draws where the next flight would go if everything went perfectly.
// It is a simulation and loops forever, so it stays thin and quiet.
func addAscent(sky *render.Sky, l sources.Launch) {
	if noAscent[l.Orbit] {
		log.Printf("no ascent arc for a %s mission", l.Orbit)
		return
	}

	inclination, ok := nominalInclination[l.Orbit]
	if !ok {
		inclination = math.Abs(l.Position.Lat)
	}
	azimuth := astro.LaunchAzimuth(l.Position.Lat, inclination)

	track := render.TrackD(astro.GreatCircle(l.Position, azimuth, ascentArcDeg, ascentStepDeg))
	if len(track) == 0 {
		log.Print("ascent arc has no drawable run")
		return
	}

	sky.Ascent = &render.Ascent{Track: track, Ride: longest(track), Seconds: ascentLoop}
	log.Printf("ascent arc from %s on azimuth %.1f for a %s mission", l.Site, azimuth, l.Orbit)
}

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
// says the day, and a rocket already climbing has no time left worth printing.
func launchWhen(l sources.Launch) string {
	switch {
	case l.Flying:
		return "lifting off"
	case l.Vague:
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
