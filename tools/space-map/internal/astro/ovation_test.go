package astro

import (
	"math"
	"slices"
	"testing"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// oval fills a grid with a band between two latitudes, over the longitudes
// given, which is the shape every test here is a variation on.
func oval(t *testing.T, low, high int, lons []int) *AuroraGrid {
	t.Helper()
	g := NewAuroraGrid(time.Now())
	for lon := range gridLons {
		for lat := -90; lat <= 90; lat++ {
			p := 0.0
			band := lat >= low && lat <= high
			if low < 0 {
				band = lat <= low && lat >= high
			}
			if band && (lons == nil || slices.Contains(lons, lon)) {
				p = 40
			}
			if err := g.Set(lon, lat, p); err != nil {
				t.Fatal(err)
			}
		}
	}
	return g
}

// A grid missing cells must not pass for a quiet sky: a hole reads as no aurora
// there, which is a bite taken out of the oval.
func TestGridRejectsGapsAndNonsense(t *testing.T) {
	g := NewAuroraGrid(time.Now())
	if err := g.Complete(); err == nil {
		t.Error("an empty grid passed as complete")
	}
	if err := g.Set(0, 91, 5); err == nil {
		t.Error("accepted a cell off the top of the grid")
	}
	if err := g.Set(360, 0, 5); err == nil {
		t.Error("accepted a longitude off the grid")
	}
	if err := g.Set(0, 0, 140); err == nil {
		t.Error("accepted a probability over 100")
	}
	if err := oval(t, 65, 72, nil).Complete(); err != nil {
		t.Errorf("a full grid was called incomplete: %v", err)
	}
}

// The drawn band is the ground the glow is seen from, so it has to reach
// further towards the equator than the oval standing overhead.
func TestFootprintReachesPastTheOvalItself(t *testing.T) {
	rings := oval(t, 65, 72, nil).Footprint(true)
	if len(rings) != 1 {
		t.Fatalf("a band on every meridian came out as %d rings, want one closed one", len(rings))
	}
	lowest, highest := 91.0, -91.0
	for _, p := range rings[0].Ring {
		lowest = math.Min(lowest, p.Lat)
		highest = math.Max(highest, p.Lat)
	}
	if lowest >= 65 {
		t.Errorf("band stops at %.1fN, no further south than the oval's own 65N", lowest)
	}
	if want := 65 - HorizonSkirt; math.Abs(lowest-want) > 1e-9 {
		t.Errorf("band stops at %.1fN, want the %.1fN horizon line", lowest, want)
	}
	if highest != 72 {
		t.Errorf("band reaches %.1fN, want the oval's poleward edge at 72N", highest)
	}
}

// A closed ring has to span the whole map, seam included, or the oval is drawn
// with a gash down the antimeridian.
func TestFootprintClosesAcrossTheSeam(t *testing.T) {
	ring := oval(t, 65, 72, nil).Footprint(true)[0]
	west, east := 181.0, -181.0
	for _, p := range ring.Ring {
		west = math.Min(west, p.Lon)
		east = math.Max(east, p.Lon)
	}
	if west != -180 || east != 180 {
		t.Errorf("ring spans %.0f..%.0f, want -180..180", west, east)
	}
}

// Aurora over half the world is half a ring, not a band closed across the side
// where there is nothing to see.
func TestFootprintBreaksWhereThereIsNoAurora(t *testing.T) {
	var half []int
	for lon := 10; lon < 100; lon++ {
		half = append(half, lon)
	}
	rings := oval(t, 65, 72, half).Footprint(true)
	if len(rings) != 1 {
		t.Fatalf("got %d rings, want the one stretch that has aurora", len(rings))
	}
	for _, p := range rings[0].Ring {
		if p.Lon < 10-taperDeg || p.Lon > 99+taperDeg {
			t.Fatalf("ring runs out to lon %.0f, past the taper on the stretch with aurora", p.Lon)
		}
	}

	// The arc has to close to a point rather than end at a wall of light.
	ends := map[float64]int{}
	for _, p := range rings[0].Ring {
		ends[p.Lon]++
	}
	for _, lon := range []float64{10 - taperDeg, 99 + taperDeg} {
		var lats []float64
		for _, p := range rings[0].Ring {
			if p.Lon == lon {
				lats = append(lats, p.Lat)
			}
		}
		if len(lats) != 2 || lats[0] != lats[1] {
			t.Errorf("at lon %.0f the band ends at %v, want both edges met at a point", lon, lats)
		}
	}
	if len(oval(t, 65, 72, []int{}).Footprint(true)) != 0 {
		t.Error("drew a band for a sky with no aurora in it")
	}
}

// The southern oval is the same shape upside down, and its skirt has to hang
// north of it rather than off the bottom of the world.
func TestSouthernFootprintHangsTowardsTheEquator(t *testing.T) {
	rings := oval(t, -65, -72, nil).Footprint(false)
	if len(rings) != 1 {
		t.Fatalf("got %d rings, want one", len(rings))
	}
	highest := -91.0
	for _, p := range rings[0].Ring {
		highest = math.Max(highest, p.Lat)
	}
	if want := -65 + HorizonSkirt; math.Abs(highest-want) > 1e-9 {
		t.Errorf("band stops at %.1f, want the %.1f horizon line", highest, want)
	}
}

// One hot cell far from the oval is noise. Anchoring on the column's strongest
// reading keeps it from dragging the whole band down over Spain.
func TestStrayCellDoesNotDragTheBandSouth(t *testing.T) {
	g := oval(t, 65, 72, nil)
	if err := g.Set(0, 40, 30); err != nil {
		t.Fatal(err)
	}
	lat, ok := g.ReachAt(0, true)
	if !ok {
		t.Fatal("lost the band at the meridian with the stray cell")
	}
	if want := 65 - HorizonSkirt; math.Abs(lat-want) > 1e-9 {
		t.Errorf("band reaches %.1fN, want %.1fN with the stray cell ignored", lat, want)
	}
}

// A few percent of glow over the polar cap is not an oval. Drawn, it becomes a
// slab: the projection stretches the pole to the map's full width.
func TestWeakPolarCapIsNotAnOval(t *testing.T) {
	g := NewAuroraGrid(time.Now())
	for lon := range gridLons {
		for lat := -90; lat <= 90; lat++ {
			p := 0.0
			if lat <= -78 {
				p = edgeProbability + 1
			}
			if err := g.Set(lon, lat, p); err != nil {
				t.Fatal(err)
			}
		}
	}
	if rings := g.Footprint(false); len(rings) != 0 {
		t.Errorf("drew %d bands for a cap with no oval in it", len(rings))
	}

	// The same cap under a real oval still has to stop short of the pole.
	for lon := range gridLons {
		for lat := -72; lat >= -76; lat-- {
			if err := g.Set(lon, lat, seedProbability+4); err != nil {
				t.Fatal(err)
			}
		}
	}
	rings := g.Footprint(false)
	if len(rings) != 1 {
		t.Fatalf("got %d bands, want the one oval", len(rings))
	}
	for _, p := range rings[0].Ring {
		if p.Lat <= -87 {
			t.Fatalf("band runs to %.0f, into the cap", p.Lat)
		}
	}
}

// The oval is strongest around midnight and peters out along itself, so a band
// carries its own strength instead of switching on at the meridian it was cut at.
func TestBandFadesAlongItsLength(t *testing.T) {
	g := NewAuroraGrid(time.Now())
	for lon := range gridLons {
		for lat := -90; lat <= 90; lat++ {
			p := 0.0
			if lat >= 65 && lat <= 72 {
				// Strong at the seam, tailing off eastwards.
				p = max(fullProbability-float64(lon)/6, 0)
			}
			if err := g.Set(lon, lat, p); err != nil {
				t.Fatal(err)
			}
		}
	}

	bands := g.Footprint(true)
	if len(bands) == 0 {
		t.Fatal("no band at all")
	}
	var strong, weak float64
	for _, band := range bands {
		for _, s := range band.Strength {
			if s.Value <= 0 {
				continue
			}
			if strong == 0 {
				strong, weak = s.Value, s.Value
			}
			strong, weak = math.Max(strong, s.Value), math.Min(weak, s.Value)
		}
	}
	if strong <= weak {
		t.Errorf("every meridian came out at strength %.2f, want the oval to fade along itself", strong)
	}
	if weak < faintest-1e-9 {
		t.Errorf("weakest meridian is %.2f, below the %.2f a drawn band starts at", weak, faintest)
	}
	if strong > 1 {
		t.Errorf("strongest meridian is %.2f, over full strength", strong)
	}
}

// The map only draws what someone could look at, so the test that decides it
// has to hold for a midsummer pole in full daylight.
func TestPolarDayLeavesNothingToSee(t *testing.T) {
	june := time.Date(2026, time.June, 21, 12, 0, 0, 0, time.UTC)
	subsolar := SubsolarPoint(june)

	var arctic []geo.Point
	for lon := -180.0; lon < 180; lon += 5 {
		arctic = append(arctic, geo.Point{Lon: lon, Lat: 84})
	}
	if AnyDark(arctic, subsolar, 0) {
		t.Error("found night at 84N in midsummer")
	}

	var antarctic []geo.Point
	for lon := -180.0; lon < 180; lon += 5 {
		antarctic = append(antarctic, geo.Point{Lon: lon, Lat: -84})
	}
	if !AnyDark(antarctic, subsolar, 0) {
		t.Error("found no night at 84S in the southern winter")
	}
}
