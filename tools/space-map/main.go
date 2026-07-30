// Command space-map renders the animated world map embedded in the profile
// README: launches, eclipses, aurora, terminator, ISS and meteor showers.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/data"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/astro"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/render"
)

func main() {
	out := flag.String("out", "dist", "directory to write space-map.svg into")
	offline := flag.Bool("offline", false, "force every network source to fail, for testing degradation")
	at := flag.String("at", "", "render the sky at an RFC3339 instant instead of now, for eyeballing other times of day")
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("space-map: ")

	now := time.Now().UTC()
	if *at != "" {
		parsed, err := time.Parse(time.RFC3339, *at)
		if err != nil {
			log.Fatalf("parse -at: %v", err)
		}
		now = parsed.UTC()
	}

	if err := run(*out, *offline, now); err != nil {
		log.Fatal(err)
	}
}

func run(outDir string, offline bool, now time.Time) error {
	sky := render.Sky{Generated: now}

	// The basemap is embedded, so this only fails if the binary is broken.
	bm, err := data.LoadBasemap()
	if err != nil {
		return err
	}
	sky.LandPath = bm.Land

	if offline {
		log.Print("offline mode, no sources fetched")
	}

	// The sun needs no network, so this layer is always present.
	sky.Terminator = buildTerminator(sky.Generated)

	sky.Legend = []render.LegendItem{
		{Colour: render.SunCore, Label: "daylight"},
	}
	sky.Ticker = []string{}

	svg := render.Document(sky)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create out dir: %w", err)
	}
	path := filepath.Join(outDir, "space-map.svg")
	if err := os.WriteFile(path, []byte(svg), 0o644); err != nil {
		return fmt.Errorf("write svg: %w", err)
	}

	log.Printf("wrote %s (%.1f KB)", path, float64(len(svg))/1024)
	return nil
}

// terminatorStepDeg is how finely the day/night boundary is traced. Two degrees
// is under three pixels of spacing on a 1000px map.
const terminatorStepDeg = 2.0

func buildTerminator(now time.Time) *render.Terminator {
	subsolar := astro.SubsolarPoint(now)
	night := astro.NightPolygon(subsolar, terminatorStepDeg)
	sun := geo.Project(subsolar)

	log.Printf("subsolar point %.2fN %.2fE", subsolar.Lat, subsolar.Lon)

	return &render.Terminator{
		NightPath: render.PolygonPath(night, 0),
		SunX:      sun.X,
		SunY:      sun.Y,
	}
}
