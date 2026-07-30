package sources

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/LarsLT/LarsLT/tools/space-map/internal/geo"
)

// feedNow is the instant testdata/launches.json is read at: after the first
// captured launch has flown, and 30 days short of the last one.
var feedNow = time.Date(2026, 7, 30, 6, 0, 0, 0, time.UTC)

func TestLaunchesParsesCapturedFeed(t *testing.T) {
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(readTestdata(t, "launches.json"))
	})

	body, err := testClient(t.TempDir()).Fetch(context.Background(), Request{
		URL: srv.URL, CacheKey: "launches", MaxAge: launchMaxAge, Valid: validLaunches,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	launches, err := parseLaunches(body, feedNow)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	want := []struct {
		name  string
		at    string
		site  string
		pos   geo.Point
		vague bool
	}{
		{"Falcon 9 Block 5 | NROL-95", "2026-07-30T07:10:19Z", "Cape Canaveral SFS",
			geo.Point{Lon: -80.57735736, Lat: 28.56194122}, false},
		{"Rocket | Out Of Feed Order", "2026-07-31T18:00:00Z", "Baikonur Cosmodrome",
			geo.Point{Lon: 63.564003, Lat: 45.996034}, false},
		{"Falcon 9 Block 5 | Starlink Group 17-52", "2026-08-01T02:00:00Z", "Vandenberg SFB",
			geo.Point{Lon: -120.611, Lat: 34.632}, false},
		{"Rocket | Second Flight From One Pad", "2026-08-04T14:00:00Z", "Vandenberg SFB",
			geo.Point{Lon: -120.611, Lat: 34.632}, false},
		{"Spectrum | Onward and Upward", "2026-08-06T00:00:00Z", "Andøya Spaceport",
			geo.Point{Lon: 15.5895, Lat: 69.1084}, true},
		{"Rocket | Unknown Precision", "2026-08-11T09:30:00Z", "Satish Dhawan Space Centre",
			geo.Point{Lon: 80.2304, Lat: 13.7199}, true},
	}

	if len(launches) != len(want) {
		t.Fatalf("parsed %d launches, want %d: %v", len(launches), len(want), names(launches))
	}
	for i, w := range want {
		got := launches[i]
		if got.Name != w.name {
			t.Errorf("launch %d is %q, want %q", i, got.Name, w.name)
		}
		if at := got.At.Format(time.RFC3339); at != w.at {
			t.Errorf("%s lifts off %s, want %s", w.name, at, w.at)
		}
		if got.Site != w.site {
			t.Errorf("%s flies from %q, want %q", w.name, got.Site, w.site)
		}
		if got.Position != w.pos {
			t.Errorf("%s sits at %v, want %v", w.name, got.Position, w.pos)
		}
		if got.Vague != w.vague {
			t.Errorf("%s vague = %v, want %v", w.name, got.Vague, w.vague)
		}
		if i > 0 && got.At.Before(launches[i-1].At) {
			t.Errorf("%s is out of order", w.name)
		}
	}

	// Two of the six share a pad, and one pad is one dot.
	if pads := Pads(launches); len(pads) != 5 {
		t.Errorf("grouped into %d pads, want 5", len(pads))
	}
}

// TestLaunchesDropsRecordsItCannotPlace covers the null fields the feed really
// sends: no pad, no T-0, and a precision this code has never seen.
func TestLaunchesDropsRecordsItCannotPlace(t *testing.T) {
	launches, err := parseLaunches(readTestdata(t, "launches.json"), feedNow)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	for _, l := range launches {
		switch {
		case l.Name == "Rocket | No Pad" || l.Name == "Rocket | No T-0":
			t.Errorf("%q should have been dropped", l.Name)
		case l.Position == (geo.Point{}):
			t.Errorf("%q was placed at the null island", l.Name)
		}
	}
}

func TestLaunchesRejectsAFeedWithoutResults(t *testing.T) {
	for _, body := range []string{`{"detail":"Request was throttled."}`, `{"results":[]}`, ``} {
		if err := validLaunches([]byte(body)); err == nil {
			t.Errorf("%q passed for a launch feed", body)
		}
	}
}

func TestVagueT0(t *testing.T) {
	for _, abbrev := range []string{"SEC", "MIN", "HR"} {
		if vagueT0(abbrev) {
			t.Errorf("%s pins the time of day", abbrev)
		}
	}
	for _, abbrev := range []string{"DAY", "M", "Q1", "Q2", "Q3", "Q4", "Y", "", "DECADE"} {
		if !vagueT0(abbrev) {
			t.Errorf("%q does not pin the time of day", abbrev)
		}
	}
}

func names(launches []Launch) []string {
	var out []string
	for _, l := range launches {
		out = append(out, l.Name)
	}
	return out
}
