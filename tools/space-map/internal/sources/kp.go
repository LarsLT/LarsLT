package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// kpURL is GFZ Potsdam's own feed, three-hourly. NOAA SWPC would be the other
// option, but it answers Actions runners with a bot wall.
const kpURL = "https://kp.gfz.de/app/json/?start=%s&end=%s&index=Kp"

// kpLookback covers a couple of days, so a gap in the latest values still
// leaves something recent to fall back on.
const kpLookback = 48 * time.Hour

// kpMaxAge is how old the newest reading may be. A cached copy outlives its own
// data, and anything older says more about the cache than about the sun.
const kpMaxAge = 24 * time.Hour

// Geomagnetic is the most recent Kp reading.
type Geomagnetic struct {
	Kp float64
	At time.Time
}

type kpFeed struct {
	// An hour GFZ has not measured comes through as null, which would decode
	// into a confident Kp 0.0 if this were a plain float.
	Kp       []*float64 `json:"Kp"`
	Datetime []string   `json:"datetime"`
}

// Kp returns the latest three-hourly planetary K index.
func Kp(ctx context.Context, c *Client, now time.Time) (Geomagnetic, error) {
	url := fmt.Sprintf(kpURL,
		now.Add(-kpLookback).UTC().Format("2006-01-02T15:04:05Z"),
		now.UTC().Format("2006-01-02T15:04:05Z"))

	body, err := c.Fetch(ctx, Request{URL: url, CacheKey: "kp", Valid: validKp})
	if err != nil {
		return Geomagnetic{}, err
	}
	return parseKp(body, now)
}

func parseKp(body []byte, now time.Time) (Geomagnetic, error) {
	kp, err := decodeKp(body)
	if err != nil {
		return Geomagnetic{}, err
	}
	if now.Sub(kp.At) > kpMaxAge {
		return Geomagnetic{}, fmt.Errorf("newest kp reading is from %s", kp.At.Format(time.RFC3339))
	}
	return kp, nil
}

func validKp(body []byte) error {
	_, err := decodeKp(body)
	return err
}

// decodeKp takes the newest reading that is a number the index can actually
// hold, skipping the nulls GFZ pads the tail of the series with.
func decodeKp(body []byte) (Geomagnetic, error) {
	var feed kpFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return Geomagnetic{}, fmt.Errorf("parse kp: %w", err)
	}
	if len(feed.Kp) == 0 || len(feed.Datetime) != len(feed.Kp) {
		return Geomagnetic{}, fmt.Errorf("kp feed has %d values and %d timestamps",
			len(feed.Kp), len(feed.Datetime))
	}

	for i := len(feed.Kp) - 1; i >= 0; i-- {
		v := feed.Kp[i]
		if v == nil || *v < 0 || *v > 9 {
			continue
		}
		at, err := time.Parse(time.RFC3339, feed.Datetime[i])
		if err != nil {
			return Geomagnetic{}, fmt.Errorf("parse kp time: %w", err)
		}
		return Geomagnetic{Kp: *v, At: at.UTC()}, nil
	}
	return Geomagnetic{}, fmt.Errorf("no usable reading among %d kp values", len(feed.Kp))
}
