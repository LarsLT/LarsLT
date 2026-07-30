// Package data holds the generated map inputs, baked into the binary. They come
// from the one-off generators under cmd/, so no build ever downloads geometry.
package data

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

//go:embed basemap.json
var basemapJSON []byte

// Basemap is the simplified world outline, already projected into the map's
// 1000x500 coordinate space as one SVG path.
type Basemap struct {
	Generated string  `json:"generated"`
	Source    string  `json:"source"`
	Tolerance float64 `json:"tolerance_deg"`
	ViewBox   []int   `json:"viewbox"`
	Rings     int     `json:"rings"`
	Land      string  `json:"land"`
}

// LoadBasemap decodes the embedded outline.
func LoadBasemap() (*Basemap, error) {
	var bm Basemap
	if err := json.Unmarshal(basemapJSON, &bm); err != nil {
		return nil, fmt.Errorf("decode basemap: %w", err)
	}
	if bm.Land == "" {
		return nil, fmt.Errorf("basemap has no land path")
	}
	return &bm, nil
}

//go:embed eclipses.json
var eclipsesJSON []byte

// Eclipse is one central solar eclipse: the shadow's two limits and the line
// down the middle, traced at 120-second steps.
type Eclipse struct {
	Date     string      `json:"date"`
	Kind     string      `json:"kind"`
	Regions  string      `json:"regions"`
	Greatest string      `json:"greatest"`
	North    []geo.Point `json:"north"`
	South    []geo.Point `json:"south"`
	Central  []geo.Point `json:"central"`
}

// When is the date the eclipse falls on, at the hour of greatest eclipse.
func (e Eclipse) When() (time.Time, error) {
	return time.Parse("2006-01-02 15:04", e.Date+" "+e.Greatest)
}

// NextEclipse returns the soonest eclipse still ahead of now.
func NextEclipse(now time.Time) (*Eclipse, error) {
	var eclipses []Eclipse
	if err := json.Unmarshal(eclipsesJSON, &eclipses); err != nil {
		return nil, fmt.Errorf("decode eclipses: %w", err)
	}
	for _, e := range eclipses {
		when, err := e.When()
		if err != nil {
			return nil, fmt.Errorf("eclipse %s: %w", e.Date, err)
		}
		// The path is drawn for the whole day it falls on, so an eclipse only
		// drops off the map once that day is over.
		if when.AddDate(0, 0, 1).After(now) {
			return &e, nil
		}
	}
	return nil, fmt.Errorf("no eclipse left in the table, regenerate data/eclipses.json")
}
