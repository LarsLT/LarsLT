package sources

import (
	"context"
	"fmt"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/astro"
)

// tleURL is Celestrak's element set for object 25544, the ISS.
const tleURL = "https://celestrak.org/NORAD/elements/gp.php?CATNR=25544&FORMAT=TLE"

// maxTLEAge is how stale an element set may be before the track is not worth
// drawing. Elements decay slowly, so a cached copy carries a build for days.
const maxTLEAge = 14 * 24 * time.Hour

// ISS returns the current element set for the station.
func ISS(ctx context.Context, c *Client, now time.Time) (*astro.TLE, error) {
	body, err := c.Fetch(ctx, tleURL, "iss")
	if err != nil {
		return nil, err
	}

	tle, err := astro.ParseTLE(string(body))
	if err != nil {
		return nil, err
	}
	if age := tle.Age(now); age > maxTLEAge || age < -time.Hour {
		return nil, fmt.Errorf("elements are %s old", age.Round(time.Hour))
	}
	return tle, nil
}
