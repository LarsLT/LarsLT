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
// drawing. This propagator reads no drag term and cannot see a reboost.
const maxTLEAge = 3 * 24 * time.Hour

// ISS returns the current element set for the station.
func ISS(ctx context.Context, c *Client, now time.Time) (*astro.TLE, error) {
	// The epoch dates the elements themselves, so the cached file's own age says
	// nothing the age check below does not already catch.
	body, err := c.Fetch(ctx, Request{URL: tleURL, CacheKey: "iss", Valid: validTLE})
	if err != nil {
		return nil, err
	}
	return parseTLE(body, now)
}

func parseTLE(body []byte, now time.Time) (*astro.TLE, error) {
	tle, err := astro.ParseTLE(string(body))
	if err != nil {
		return nil, err
	}
	if age := tle.Age(now); age > maxTLEAge || age < -time.Hour {
		return nil, fmt.Errorf("elements are %s old", age.Round(time.Hour))
	}
	return tle, nil
}

// validTLE keeps Celestrak's over-quota answer, a bare "No GP data found", from
// reaching the cache.
func validTLE(body []byte) error {
	_, err := astro.ParseTLE(string(body))
	return err
}
