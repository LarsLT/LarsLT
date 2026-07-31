package sources

import (
	_ "embed"
	"fmt"
	"time"
)

// seedLaunches is a real upcoming feed, trimmed to the fields the map reads and
// committed so a run that finds no cache still has a launch layer to draw.

//go:embed seed/launches.json
var seedLaunches []byte

// seedTaken is when that copy was fetched. A checkout stamps every file with
// the moment it was written, so nothing on disk can answer this.
const seedTaken = "2026-07-31T08:37:06Z"

// launchSeed hands the committed feed back while it is still worth believing.
// The age rule is the cache's: a day on, too many of these T-0s have slipped.
func launchSeed(now time.Time) ([]byte, error) {
	taken, err := time.Parse(time.RFC3339, seedTaken)
	if err != nil {
		return nil, fmt.Errorf("seed is dated %q: %w", seedTaken, err)
	}
	if age := now.Sub(taken); age > launchMaxAge {
		return nil, fmt.Errorf("the committed seed is %s old", age.Round(time.Hour))
	}
	return seedLaunches, nil
}
