package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// launchesURL asks for enough launches to cover a month. The upcoming feed is
// ordered by T-0, so a slice off the front is all we need.
const launchesURL = "https://ll.thespacedevs.com/2.3.0/launches/upcoming/?limit=60"

// LaunchWindow is how far ahead a launch still counts as upcoming.
const LaunchWindow = 30 * 24 * time.Hour

// launchMaxAge caps how long the cached feed may stand in for the live one. A
// T-0 is a plan: after a day of outage too many of them have quietly slipped.
const launchMaxAge = 24 * time.Hour

// Launch is one upcoming flight, reduced to what the map draws.
type Launch struct {
	Name     string
	Provider string
	Pad      string
	Site     string
	At       time.Time
	Vague    bool // T-0 is known only to the day or worse
	Position geo.Point
}

type launchFeed struct {
	Results []struct {
		Name      string `json:"name"`
		Net       string `json:"net"`
		Precision struct {
			Abbrev string `json:"abbrev"`
		} `json:"net_precision"`
		Provider struct {
			Abbrev string `json:"abbrev"`
			Name   string `json:"name"`
		} `json:"launch_service_provider"`
		Pad struct {
			Name      string   `json:"name"`
			Latitude  *float64 `json:"latitude"`
			Longitude *float64 `json:"longitude"`
			Location  struct {
				Name string `json:"name"`
			} `json:"location"`
		} `json:"pad"`
	} `json:"results"`
}

// Launches returns the flights lifting off between now and the window's end,
// soonest first.
func Launches(ctx context.Context, c *Client, now time.Time) ([]Launch, error) {
	body, err := c.Fetch(ctx, Request{
		URL:      launchesURL,
		CacheKey: "launches",
		MaxAge:   launchMaxAge,
		Valid:    validLaunches,
	})
	if err != nil {
		return nil, err
	}
	return parseLaunches(body, now)
}

func validLaunches(body []byte) error {
	_, err := decodeLaunches(body)
	return err
}

// decodeLaunches treats an empty result set as a refusal. The upcoming feed
// always has hundreds of entries, so nothing at all is upstream saying no.
func decodeLaunches(body []byte) (*launchFeed, error) {
	var feed launchFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse launches: %w", err)
	}
	if len(feed.Results) == 0 {
		return nil, fmt.Errorf("launch feed carried no results")
	}
	return &feed, nil
}

func parseLaunches(body []byte, now time.Time) ([]Launch, error) {
	feed, err := decodeLaunches(body)
	if err != nil {
		return nil, err
	}

	cutoff := now.Add(LaunchWindow)
	var out []Launch
	for _, r := range feed.Results {
		at, err := time.Parse(time.RFC3339, r.Net)
		if err != nil || at.Before(now) || at.After(cutoff) {
			continue
		}
		if r.Pad.Latitude == nil || r.Pad.Longitude == nil {
			continue
		}
		out = append(out, Launch{
			Name:     r.Name,
			Provider: firstNonEmpty(r.Provider.Abbrev, r.Provider.Name),
			Pad:      r.Pad.Name,
			Site:     shortSite(r.Pad.Location.Name),
			At:       at.UTC(),
			Vague:    vagueT0(r.Precision.Abbrev),
			Position: geo.Point{Lon: *r.Pad.Longitude, Lat: *r.Pad.Latitude},
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// vagueT0 says whether net_precision leaves the time of day unknown. The three
// precise values are named, so anything new upstream invents counts as vague.
func vagueT0(abbrev string) bool {
	switch abbrev {
	case "SEC", "MIN", "HR":
		return false
	}
	return true
}

// Pad is a launch site with at least one flight in the window. Several launches
// share a pad, and the map only needs the dot once.
type Pad struct {
	Position geo.Point
	Next     Launch
}

// Pads groups launches by site, soonest first.
func Pads(launches []Launch) []Pad {
	var pads []Pad
	seen := map[string]bool{}
	for _, l := range launches {
		// Neighbouring pads at one spaceport land on the same pixel, so the key
		// is the rounded position rather than the pad name.
		key := fmt.Sprintf("%.1f,%.1f", l.Position.Lat, l.Position.Lon)
		if seen[key] {
			continue
		}
		seen[key] = true
		pads = append(pads, Pad{Position: l.Position, Next: l})
	}
	return pads
}

// shortSite trims the country off a Launch Library site name, which reads
// "Cape Canaveral SFS, FL, USA".
func shortSite(name string) string {
	if name == "" {
		return ""
	}
	if i := strings.Index(name, ","); i > 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
