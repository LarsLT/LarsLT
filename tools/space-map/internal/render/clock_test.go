package render

import (
	"math"
	"testing"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

var epoch = time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)

func TestPhaseDelay(t *testing.T) {
	cases := []struct {
		name   string
		period time.Duration
		offset time.Duration
		want   float64
	}{
		{"start of the period", time.Hour, 0, 0},
		{"part way through", time.Hour, 15 * time.Minute, -900},
		{"exactly one period", time.Hour, time.Hour, 0},
		{"several periods on", time.Hour, 7*time.Hour + 30*time.Minute, -1800},
		{"before the epoch", time.Hour, -15 * time.Minute, -2700},
		{"a solar day", SolarDay, 6 * time.Hour, -21600},
		{"no period", 0, time.Hour, 0},
		{"negative period", -time.Hour, time.Hour, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := PhaseDelay(c.period, epoch, epoch.Add(c.offset)); got != c.want {
				t.Errorf("PhaseDelay = %v, want %v", got, c.want)
			}
		})
	}
}

func TestPhaseDelayStaysInsideThePeriod(t *testing.T) {
	// The widest gap two instants can have, over the shortest period there is:
	// that quotient is where a float-to-int cast stops being defined.
	far := epoch.Add(math.MaxInt64)
	for _, period := range []time.Duration{time.Nanosecond, time.Microsecond, time.Second, SolarDay} {
		got := PhaseDelay(period, epoch, far)
		if got > 0 || got <= -period.Seconds() {
			t.Errorf("PhaseDelay(%v) = %v, outside (-%v, 0]", period, got, period.Seconds())
		}
	}
}

// leg is a track sampled every step, in the shape the ISS layer builds.
func leg(step time.Duration, pts ...geo.Point) ([]geo.Point, []time.Time) {
	times := make([]time.Time, len(pts))
	for i := range pts {
		times[i] = epoch.Add(time.Duration(i) * step)
	}
	return pts, times
}

func TestArcPhaseDelayFollowsArcLengthNotTime(t *testing.T) {
	// Three quarters of the length covered in half the time: the equator sprint
	// followed by the turn at maximum latitude, exaggerated.
	track, times := leg(100*time.Second,
		geo.Point{Lon: -180, Lat: 0}, // x = 0
		geo.Point{Lon: -72, Lat: 0},  // x = 300
		geo.Point{Lon: -36, Lat: 0},  // x = 400
	)
	const span = 200.0

	cases := []struct {
		offset time.Duration
		want   float64
	}{
		{0, 0},
		{50 * time.Second, -75},
		{100 * time.Second, -150},
		{150 * time.Second, -175},
		{200 * time.Second, 0},     // the leg loops
		{250 * time.Second, -75},   // and so does anything past it
		{-50 * time.Second, -175},  // or before it
		{-250 * time.Second, -175}, // however far before
	}
	for _, c := range cases {
		got := ArcPhaseDelay(track, times, epoch.Add(c.offset))
		if math.Abs(got-c.want) > 1e-9 {
			t.Errorf("ArcPhaseDelay(+%v) = %v, want %v", c.offset, got, c.want)
		}
		if got > 0 || got <= -span {
			t.Errorf("ArcPhaseDelay(+%v) = %v, outside (-%v, 0]", c.offset, got, span)
		}
	}

	// The point of the change: time alone puts the marker somewhere else.
	if timed := PhaseDelay(200*time.Second, epoch, epoch.Add(100*time.Second)); timed == -150 {
		t.Fatal("the time-based delay agrees, so this leg does not exercise the bug")
	}
}

func TestArcPhaseDelayDegenerate(t *testing.T) {
	track, times := leg(100*time.Second, geo.Point{Lon: 0, Lat: 0}, geo.Point{Lon: 10, Lat: 0})

	cases := []struct {
		name  string
		track []geo.Point
		times []time.Time
	}{
		{"nothing at all", nil, nil},
		{"one point", track[:1], times[:1]},
		{"more points than times", track, times[:1]},
		{"more times than points", track[:1], times},
		{"a leg of no length", []geo.Point{track[0], track[0]}, times},
		{"a leg of no duration", track, []time.Time{epoch, epoch}},
		{"time running backwards", track, []time.Time{times[1], times[0]}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ArcPhaseDelay(c.track, c.times, epoch.Add(time.Minute)); got != 0 {
				t.Errorf("ArcPhaseDelay = %v, want 0", got)
			}
		})
	}
}

func TestArcPhaseDelaySurvivesUnorderedTimes(t *testing.T) {
	// A source that hands back a scrambled track must not take the build down.
	track, _ := leg(0, geo.Point{Lon: -10, Lat: 20}, geo.Point{Lon: 0, Lat: 40}, geo.Point{Lon: 10, Lat: 0})
	times := []time.Time{epoch, epoch.Add(100 * time.Second), epoch.Add(50 * time.Second)}
	got := ArcPhaseDelay(track, times, epoch.Add(30*time.Second))
	if got > 0 || got <= -50 || math.IsNaN(got) {
		t.Errorf("ArcPhaseDelay = %v, want something inside (-50, 0]", got)
	}
}

func TestArcPhaseDelayLandsOnTheRightPixel(t *testing.T) {
	// A ground track shaped like the real one: longitude marching on while
	// latitude swings, so the projected speed changes the whole way along.
	const (
		step    = 30 * time.Second
		samples = 91
		period  = 5580.0
	)
	track := make([]geo.Point, samples)
	times := make([]time.Time, samples)
	for i := range track {
		sec := float64(i) * step.Seconds()
		track[i] = geo.Point{
			Lon: -170 + sec*360/period,
			Lat: 51.6 * math.Sin(2*math.Pi*sec/period),
		}
		times[i] = epoch.Add(time.Duration(i) * step)
	}
	span := times[samples-1].Sub(times[0])

	var worstArc, worstTimed float64
	for sec := 0.0; sec < span.Seconds(); sec += 5 {
		now := epoch.Add(time.Duration(sec) * time.Second)
		truth := trueAt(track, times, now)
		worstArc = math.Max(worstArc, dist(atFraction(track, -ArcPhaseDelay(track, times, now)/span.Seconds()), truth))
		worstTimed = math.Max(worstTimed, dist(atFraction(track, -PhaseDelay(span, times[0], now)/span.Seconds()), truth))
	}

	if worstArc > 0.01 {
		t.Errorf("arc-length phase is up to %.2f px off the true position", worstArc)
	}
	if worstTimed < 10 {
		t.Errorf("time-based phase is only %.2f px off, so this track does not show the bug", worstTimed)
	}
	t.Logf("worst error over the leg: arc %.3f px, time-based %.3f px", worstArc, worstTimed)
}

// trueAt is where the station actually is: linear between the two samples that
// bracket now, which is the best the sampled track can say.
func trueAt(track []geo.Point, times []time.Time, now time.Time) geo.XY {
	for i := 0; i+1 < len(track); i++ {
		if !now.After(times[i+1]) {
			f := now.Sub(times[i]).Seconds() / times[i+1].Sub(times[i]).Seconds()
			a, b := geo.Project(track[i]), geo.Project(track[i+1])
			return geo.XY{X: a.X + (b.X-a.X)*f, Y: a.Y + (b.Y-a.Y)*f}
		}
	}
	return geo.Project(track[len(track)-1])
}

// atFraction is where the browser puts a marker at a given offset-distance.
func atFraction(track []geo.Point, f float64) geo.XY {
	var total float64
	for i := 0; i+1 < len(track); i++ {
		total += dist(geo.Project(track[i]), geo.Project(track[i+1]))
	}
	target := f * total
	var walked float64
	for i := 0; i+1 < len(track); i++ {
		a, b := geo.Project(track[i]), geo.Project(track[i+1])
		length := dist(a, b)
		if walked+length >= target {
			g := (target - walked) / length
			return geo.XY{X: a.X + (b.X-a.X)*g, Y: a.Y + (b.Y-a.Y)*g}
		}
		walked += length
	}
	return geo.Project(track[len(track)-1])
}

func dist(a, b geo.XY) float64 { return math.Hypot(b.X-a.X, b.Y-a.Y) }
