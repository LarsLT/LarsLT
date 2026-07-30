// Command space-map renders the animated world map embedded in the profile
// README: rocket launches, eclipse paths, aurora, the day/night terminator,
// the ISS ground track and active meteor showers.
//
//	go run . -out dist          # normal build
//	go run . -out dist -offline # every source forced to fail
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/data"
	"github.com/LarsLT/LarsLT/tools/space-map/internal/render"
)

func main() {
	out := flag.String("out", "dist", "directory to write space-map.svg into")
	offline := flag.Bool("offline", false, "force every network source to fail, for testing degradation")
	flag.Parse()

	log.SetFlags(0)
	log.SetPrefix("space-map: ")

	if err := run(*out, *offline); err != nil {
		log.Fatal(err)
	}
}

func run(outDir string, offline bool) error {
	sky := render.Sky{Generated: time.Now().UTC()}

	// The basemap is embedded, so this only fails if the binary is broken.
	bm, err := data.LoadBasemap()
	if err != nil {
		return err
	}
	sky.LandPath = bm.Land

	if offline {
		log.Print("offline mode, no sources fetched")
	}

	sky.Legend = []render.LegendItem{}
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
