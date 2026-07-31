package sources

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// ovationBody builds a feed the way SWPC publishes one, optionally short of the
// last cell so the incomplete case can be tested.
func ovationBody(forecast string, whole bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, `{"Forecast Time":"%s","coordinates":[`, forecast)
	first := true
	for lon := range 360 {
		for lat := -90; lat <= 90; lat++ {
			if !whole && lon == 359 && lat == 90 {
				continue
			}
			if !first {
				b.WriteString(",")
			}
			first = false
			p := 0
			if lat >= 65 && lat <= 72 {
				p = 40
			}
			fmt.Fprintf(&b, "[%d,%d,%d]", lon, lat, p)
		}
	}
	b.WriteString("]}")
	return b.String()
}

func TestOvationReadsTheForecastGrid(t *testing.T) {
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	grid, err := parseOvation([]byte(ovationBody("2026-07-31T09:27:00Z", true)), now)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if at := grid.Forecast.Format(time.RFC3339); at != "2026-07-31T09:27:00Z" {
		t.Errorf("forecast is for %s, want 2026-07-31T09:27:00Z", at)
	}
	if rings := grid.Footprint(true); len(rings) != 1 {
		t.Errorf("got %d bands from a grid with one oval in it", len(rings))
	}
}

// A grid describes one particular hour. An old one, live or out of the cache,
// draws last night's aurora over tonight's map.
func TestOvationRefusesAGridItCannotUse(t *testing.T) {
	now := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		body string
	}{
		{"stale", ovationBody("2026-07-30T21:00:00Z", true)},
		{"missing cells", ovationBody("2026-07-31T09:27:00Z", false)},
		{"no forecast time", `{"coordinates":[[0,0,0]]}`},
		{"not json", `<html>bot wall</html>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseOvation([]byte(tc.body), now); err == nil {
				t.Error("accepted it")
			}
		})
	}
}
