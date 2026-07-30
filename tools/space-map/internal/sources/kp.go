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

// Geomagnetic is the most recent Kp reading.
type Geomagnetic struct {
	Kp   float64
	At   time.Time
	Firm bool // a definitive value rather than GFZ's preliminary estimate
}

type kpFeed struct {
	Kp       []float64 `json:"Kp"`
	Datetime []string  `json:"datetime"`
	Status   []string  `json:"status"`
}

// Kp returns the latest three-hourly planetary K index.
func Kp(ctx context.Context, c *Client, now time.Time) (Geomagnetic, error) {
	url := fmt.Sprintf(kpURL,
		now.Add(-kpLookback).UTC().Format("2006-01-02T15:04:05Z"),
		now.UTC().Format("2006-01-02T15:04:05Z"))

	body, err := c.Fetch(ctx, url, "kp")
	if err != nil {
		return Geomagnetic{}, err
	}

	var feed kpFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return Geomagnetic{}, fmt.Errorf("parse kp: %w", err)
	}
	if len(feed.Kp) == 0 || len(feed.Datetime) != len(feed.Kp) {
		return Geomagnetic{}, fmt.Errorf("kp feed carried no usable readings")
	}

	last := len(feed.Kp) - 1
	at, err := time.Parse(time.RFC3339, feed.Datetime[last])
	if err != nil {
		return Geomagnetic{}, fmt.Errorf("parse kp time: %w", err)
	}

	// A cached copy outlives its own data. Anything older than a day says more
	// about the cache than about the sun.
	if now.Sub(at) > 24*time.Hour {
		return Geomagnetic{}, fmt.Errorf("newest kp reading is from %s", at.Format(time.RFC3339))
	}

	firm := last < len(feed.Status) && feed.Status[last] == "def"
	return Geomagnetic{Kp: feed.Kp[last], At: at.UTC(), Firm: firm}, nil
}
