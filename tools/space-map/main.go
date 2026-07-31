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

// auroraSide is one hemisphere's footprint: the ground the glow can be seen
// from, and how far towards the equator that reaches over a given meridian.
type auroraSide struct {
	north bool
	rings [][]geo.Point
	reach func(lon float64) (float64, bool)
}

// addAurora draws the ground tonight's aurora can be seen from. The renderer
// masks it to the dark side, so only the half anyone could look at shows.
func addAurora(ctx context.Context, client *sources.Client, sky *render.Sky, now time.Time) {
	kp, kpErr := sources.Kp(ctx, client, now)
	if kpErr != nil {
		log.Printf("kp unavailable: %v", kpErr)
	}

	sides, err := auroraFootprint(ctx, client, now, kp, kpErr)
	if err != nil {
		log.Printf("aurora unavailable: %v", err)
		return
	}

	// An oval sitting entirely in daylight is one nobody on Earth can see, which
	// is most of a polar summer. Then the map says nothing about aurora at all.
	subsolar := astro.SubsolarPoint(now)
	reach, seen := 91.0, false
	for _, side := range sides {
		for _, ring := range side.rings {
			if !astro.AnyDark(ring, subsolar) {
				continue
			}
			sky.Aurora = append(sky.Aurora, render.Aurora{
				Path:  render.PolygonPath(ring),
				North: side.north,
				Skirt: skirtFraction(ring),
			})
			if side.north {
				reach = min(reach, darkestReach(ring, subsolar))
				seen = true
			}
		}
	}
	if len(sky.Aurora) == 0 {
		log.Print("the whole auroral oval is in daylight, nothing anyone can see")
		return
	}

	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.AuroraCore, Label: "aurora"})
	sky.Ticker = append(sky.Ticker, auroraLine(kp, kpErr == nil, reach, seen, homeReach(sides, subsolar)))
	over := "no band"
	if lat, ok := sides[0].reach(homeLon); ok {
		over = fmt.Sprintf("%.1fN", lat)
	}
	log.Printf("aurora down to %.1fN at midnight, %s over the home meridian, %d bands drawn",
		reach, over, len(sky.Aurora))
}

// auroraFootprint prefers the forecast grid, which has the oval's real lopsided
// shape, and falls back to a circle drawn from the last Kp reading.
func auroraFootprint(ctx context.Context, client *sources.Client, now time.Time,
	kp sources.Geomagnetic, kpErr error,
) ([]auroraSide, error) {
	grid, err := sources.AuroraForecast(ctx, client, now)
	if err == nil {
		log.Printf("aurora forecast for %s", grid.Forecast.Format(time.RFC3339))
		return []auroraSide{
			{north: true, rings: grid.Footprint(true), reach: reachOf(grid, true)},
			{north: false, rings: grid.Footprint(false), reach: reachOf(grid, false)},
		}, nil
	}
	log.Printf("aurora forecast unavailable: %v", err)
	if kpErr != nil {
		return nil, kpErr
	}

	log.Printf("falling back to the Kp %.2f oval", kp.Kp)
	var sides []auroraSide
	for _, north := range []bool{true, false} {
		sides = append(sides, auroraSide{
			north: north,
			rings: [][]geo.Point{astro.Oval(kp.Kp, north, ovalStepDeg)},
			reach: func(lon float64) (float64, bool) {
				return astro.GeographicLatAt(astro.VisibleFrom(kp.Kp), lon, north), true
			},
		})
	}
	return sides, nil
}

func reachOf(grid *astro.AuroraGrid, north bool) func(float64) (float64, bool) {
	return func(lon float64) (float64, bool) { return grid.ReachAt(lon, north) }
}

// skirtFraction is how much of a band's height is ground the glow is only seen
// from rather than stands over, which is where the renderer fades it out.
func skirtFraction(ring []geo.Point) float64 {
	low, high := 91.0, -91.0
	for _, p := range ring {
		low, high = min(low, p.Lat), max(high, p.Lat)
	}
	if high-low <= 0 {
		return 0
	}
	return astro.HorizonSkirt / (high - low)
}

// darkestReach is how far towards the equator this band gets on the night side,
// which is the part of it the mask lets through.
func darkestReach(ring []geo.Point, subsolar geo.Point) float64 {
	lowest := 91.0
	for _, p := range ring {
		if astro.SunElevation(p, subsolar) < 0 {
			lowest = min(lowest, p.Lat)
		}
	}
	return lowest
}

// homeReach answers the question a Dutch profile is really asking: could you
// step outside tonight and see it. That needs the glow overhead and darkness.
func homeReach(sides []auroraSide, subsolar geo.Point) bool {
	home := geo.Point{Lon: homeLon, Lat: dutchLat}
	if astro.SunElevation(home, subsolar) >= 0 {
		return false
	}
	for _, side := range sides {
		if !side.north {
			continue
		}
		if lat, ok := side.reach(homeLon); ok && lat <= dutchLat {
			return true
		}
	}
	return false
}

// auroraLine turns the numbers into the thing people actually want to know.
func auroraLine(kp sources.Geomagnetic, haveKp bool, reach float64, seen, home bool) string {
	// The reach is the band's southernmost dark point, which sits in whatever
	// sector midnight is over, so the words must not read as "here".
	where := "over the far south"
	switch {
	case home:
		where = "out over the Netherlands"
	case seen:
		where = fmt.Sprintf("down to %.0fN at midnight", reach)
	}
	if !haveKp {
		return fmt.Sprintf("aurora %s", where)
	}
	return fmt.Sprintf("Kp %.1f  ·  aurora %s", kp.Kp, where)
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

	addAscent(sky, pads[0].Next, now)

	log.Printf("%d launches from %d pads, next %s", len(launches), len(pads), launches[0].At.Format(time.RFC3339))
}

// tickerLaunches is how many flights fit under the map without crowding it, now
// that every other layer wants a line too.
const tickerLaunches = 1

// The ascent arc: roughly the ground distance a rocket covers before insertion,
// sampled fine enough to stay smooth, and climbed once in seconds.
const (
	ascentArcDeg  = 25
	ascentStepDeg = 1
	ascentClimb   = 7
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

// rangeCorridor is the span of azimuths a spaceport may fly, a safety decision
// no formula recovers. A range not listed here keeps what the formula said.
var rangeCorridor = map[string]astro.Corridor{
	"Vandenberg SFB":          {From: 140, To: 201},
	"Cape Canaveral SFS":      {From: 37, To: 112},
	"Kennedy Space Center":    {From: 37, To: 112},
	"Wallops Flight Facility": {From: 90, To: 160},
	"Guiana Space Centre":     {From: 349, To: 93},
}

// aim is the bearing the next flight leaves on. Both roots of the azimuth reach
// the orbit, so where a range is known the one it permits wins.
func aim(l sources.Launch, inclination float64) float64 {
	azimuth := astro.LaunchAzimuth(l.Position.Lat, inclination)

	corridor, known := rangeCorridor[l.Site]
	if !known || corridor.Contains(azimuth) {
		return azimuth
	}
	if mirror := astro.MirrorAzimuth(azimuth); corridor.Contains(mirror) {
		return mirror
	}

	// Neither root is allowed, which is the minimum-energy case at a range that
	// cannot fly it. The closest permitted heading is the honest answer.
	return corridor.Nearest(azimuth)
}

// addAscent draws where the next flight would go if everything went perfectly:
// on the day of the launch, and moving only once the feed says it has left.
func addAscent(sky *render.Sky, l sources.Launch, now time.Time) {
	if noAscent[l.Orbit] {
		log.Printf("no ascent arc for a %s mission", l.Orbit)
		return
	}
	if !sameDay(l.At, now) {
		log.Printf("next launch is not today, no ascent arc")
		return
	}

	inclination, ok := nominalInclination[l.Orbit]
	if !ok {
		inclination = math.Abs(l.Position.Lat)
	}
	azimuth := aim(l, inclination)

	track := render.TrackD(astro.GreatCircle(l.Position, azimuth, ascentArcDeg, ascentStepDeg))
	if len(track) == 0 {
		log.Print("ascent arc has no drawable run")
		return
	}

	sky.Ascent = &render.Ascent{Track: track, Seconds: ascentClimb}
	label := "nominal ascent, today"
	if l.Flying {
		sky.Ascent.Ride = longest(track)
		label = "ascent under way"
	}
	sky.Legend = append(sky.Legend, render.LegendItem{Colour: render.LaunchNext, Label: label})
	log.Printf("ascent arc from %s on azimuth %.1f for a %s mission, flying %v",
		l.Site, azimuth, l.Orbit, l.Flying)
}

// sameDay is the whole of the launch's UTC date. A day-precision T-0 knows no
// more than that, so the arc lives exactly as long as the date does.
func sameDay(at, now time.Time) bool {
	a, b := at.UTC(), now.UTC()
	return a.Year() == b.Year() && a.YearDay() == b.YearDay()
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
