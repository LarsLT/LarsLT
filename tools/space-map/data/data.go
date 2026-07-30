// Package data holds the generated map inputs, baked into the binary.
//
// These files are produced by the one-off generators under cmd/ and committed.
// The scheduled build never downloads geometry.
package data

import (
	_ "embed"
	"encoding/json"
	"fmt"
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
