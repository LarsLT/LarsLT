package render

import (
	"math"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// SolarDay is one turn of the Earth relative to the sun, which is what the
// terminator animation has to match.
const SolarDay = 24 * time.Hour

// PhaseDelay returns the negative animation-delay, in seconds, that starts a
// loop part-way through, so the map keeps tracking reality between rebuilds.
func PhaseDelay(period time.Duration, epoch, now time.Time) float64 {
	if period <= 0 {
		return 0
	}
	return -wrap(now.Sub(epoch).Seconds(), period.Seconds())
}

// ArcPhaseDelay is PhaseDelay for a marker riding a track. CSS measures
// offset-distance in arc length, and a ground track covers it unevenly.
func ArcPhaseDelay(track []geo.Point, times []time.Time, now time.Time) float64 {
	if len(track) < 2 || len(times) != len(track) {
		return 0
	}
	start := times[0]
	span := times[len(times)-1].Sub(start).Seconds()
	if span <= 0 {
		return 0
	}
	// The leg loops, so a now just past its end is a now just past its start.
	elapsed := wrap(now.Sub(start).Seconds(), span)

	// Walking the polyline the browser walks: the ISS's projected speed swings by
	// more than half over a leg, which is tens of pixels of error taken on time.
	var total, walked float64
	done := false
	for i := 0; i+1 < len(track); i++ {
		a, b := geo.Project(track[i]), geo.Project(track[i+1])
		length := math.Hypot(b.X-a.X, b.Y-a.Y)
		total += length
		if done {
			continue
		}
		from, to := times[i].Sub(start).Seconds(), times[i+1].Sub(start).Seconds()
		switch {
		case elapsed >= to:
			walked += length
		case to > from:
			walked += length * (elapsed - from) / (to - from)
			done = true
		default:
			// Times out of order: stop rather than guess.
			done = true
		}
	}
	if total == 0 {
		return 0
	}
	return -walked / total * span
}

// wrap folds elapsed seconds into one period. Mod rather than a float-to-int
// cast, which is undefined once the quotient outgrows an int.
func wrap(elapsed, period float64) float64 {
	phase := math.Mod(elapsed, period)
	if phase < 0 {
		phase += period
	}
	return phase
}
