package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/astro"
)

// ovationURL is SWPC's OVATION model: an aurora probability for every whole
// degree of the globe, reissued every half hour and forecast an hour out.
const ovationURL = "https://services.swpc.noaa.gov/json/ovation_aurora_latest.json"

// maxForecastAge is how stale a grid may be. It describes one particular hour,
// so an old one draws last night's aurora over tonight's map.
const maxForecastAge = 3 * time.Hour

type ovationFeed struct {
	Forecast string `json:"Forecast Time"`
	// Each entry is longitude, latitude, probability, in that order.
	Coordinates [][3]float64 `json:"coordinates"`
}

// AuroraForecast returns the aurora probability grid for the coming hour.
func AuroraForecast(ctx context.Context, c *Client, now time.Time) (*astro.AuroraGrid, error) {
	body, err := c.Fetch(ctx, Request{URL: ovationURL, CacheKey: "ovation", Valid: validOvation})
	if err != nil {
		return nil, err
	}
	return parseOvation(body, now)
}

func parseOvation(body []byte, now time.Time) (*astro.AuroraGrid, error) {
	grid, err := decodeOvation(body)
	if err != nil {
		return nil, err
	}
	if age := now.Sub(grid.Forecast); age > maxForecastAge {
		return nil, fmt.Errorf("aurora forecast is for %s", grid.Forecast.Format(time.RFC3339))
	}
	return grid, nil
}

func validOvation(body []byte) error {
	_, err := decodeOvation(body)
	return err
}

func decodeOvation(body []byte) (*astro.AuroraGrid, error) {
	var feed ovationFeed
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse aurora forecast: %w", err)
	}
	at, err := time.Parse(time.RFC3339, feed.Forecast)
	if err != nil {
		return nil, fmt.Errorf("parse aurora forecast time: %w", err)
	}

	grid := astro.NewAuroraGrid(at.UTC())
	for _, cell := range feed.Coordinates {
		if err := grid.Set(int(cell[0]), int(cell[1]), cell[2]); err != nil {
			return nil, err
		}
	}
	if err := grid.Complete(); err != nil {
		return nil, fmt.Errorf("aurora forecast incomplete: %w", err)
	}
	return grid, nil
}
