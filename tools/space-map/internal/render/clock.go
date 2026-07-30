package render

import "time"

// SolarDay is one turn of the Earth relative to the sun, which is what the
// terminator animation has to match.
const SolarDay = 24 * time.Hour

// PhaseDelay returns the negative animation-delay, in seconds, that starts a
// loop already part-way through.
//
// This is what keeps the map honest between rebuilds. An animation whose
// duration equals the real period of the thing it shows, offset by how much of
// that period has already elapsed, keeps tracking reality on its own. The cron
// only re-syncs the phase and refreshes the data.
func PhaseDelay(period time.Duration, epoch, now time.Time) float64 {
	if period <= 0 {
		return 0
	}
	elapsed := now.Sub(epoch).Seconds()
	p := period.Seconds()
	phase := elapsed - p*float64(int(elapsed/p))
	if phase < 0 {
		phase += p
	}
	return -phase
}
