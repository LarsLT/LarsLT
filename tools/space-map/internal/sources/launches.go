package sources

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// launchesURL asks for enough launches to cover a month. The upcoming feed is
// ordered by T-0, so a slice off the front is all we need.
const launchesURL = "https://ll.thespacedevs.com/2.3.0/launches/upcoming/?limit=60"

// launchesMirrorURL is Launch Library's development endpoint: the same feed off
// a cache, without the hourly ceiling a shared runner address has already spent.
const launchesMirrorURL = "https://lldev.thespacedevs.com/2.3.0/launches/upcoming/?limit=60"

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
	Vague    bool   // T-0 is known only to the day or worse
	Flying   bool   // airborne right now, so the T-0 has gone stale
	Orbit    string // where it is headed, as the feed's abbreviation
	Position geo.Point
}

type launchFeed struct {
	Results []struct {
		Name      string `json:"name"`
		Net       string `json:"net"`
		Precision struct {
			Abbrev string `json:"abbrev"`
		} `json:"net_precision"`
		Status struct {
			Abbrev string `json:"abbrev"`
		} `json:"status"`
		Provider struct {
			Abbrev string `json:"abbrev"`
			Name   string `json:"name"`
		} `json:"launch_service_provider"`
		Mission struct {
			Orbit struct {
				Abbrev string `json:"abbrev"`
			} `json:"orbit"`
		} `json:"mission"`
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
		Mirror:   launchesMirrorURL,
		CacheKey: "launches",
		MaxAge:   launchMaxAge,
		Valid:    validLaunches,
	})
	return launchesFrom(body, err, now)
}

// launchesFrom falls back to the committed seed when every source refused, so a
// run that finds a cold cache still draws the layer.
func launchesFrom(body []byte, err error, now time.Time) ([]Launch, error) {
	if err != nil {
		// Offline is the flag asking for the degraded map, and a seed drawn anyway
		// is the one thing that would hide it.
		if errors.Is(err, ErrOffline) {
			return nil, err
		}
		seed, seedErr := launchSeed(now)
		if seedErr != nil {
			return nil, fmt.Errorf("%w, and %v", err, seedErr)
		}
		log.Printf("launches failed (%v), falling back to the committed seed", err)
		body = seed
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
		if err != nil || at.After(cutoff) {
			continue
		}
		flying := r.Status.Abbrev == statusInFlight
		if at.Before(now) && !(flying && now.Sub(at) < ascentWindow) {
			continue
		}
		if !worthDrawing(r.Status.Abbrev) {
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
			Vague:    vagueT0(r.Precision.Abbrev) || r.Status.Abbrev == statusHold,
			Flying:   flying,
			Orbit:    r.Mission.Orbit.Abbrev,
			Position: geo.Point{Lon: *r.Pad.Longitude, Lat: *r.Pad.Latitude},
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

// statusHold is a stopped countdown. The T-0 in the feed is whatever it read
// when the clock stopped, so the day is all of it still worth printing.
const statusHold = "Hold"

// statusInFlight is a rocket that is climbing as the map is built. Its T-0 is
// the plan it flew to, so by now the time is behind us and only the fact is left.
const statusInFlight = "In Flight"

// ascentWindow is how long a past T-0 may still be drawn on the In Flight status
// alone. The bound is what stops a stale cache flying a rocket for a day.
const ascentWindow = 30 * time.Minute

// worthDrawing says whether the map should still show the flight. The upcoming
// feed carries a launch through liftoff and holds on to it for hours afterwards.
func worthDrawing(status string) bool {
	switch status {
	case "Success", "Failure", "Partial Failure":
		return false
	}
	// Anything upstream invents counts as pending: a launch drawn a little too
	// long is a smaller lie than a map that quietly loses the layer.
	return true
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
