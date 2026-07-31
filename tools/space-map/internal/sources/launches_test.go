package sources

import (
	"context"
	"errors"
	"fmt"
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
		orbit string
	}{
		// NROL-95 is captured mid-flight: its T-0 is still ahead of feedNow, but
		// the feed already calls it a success, so the map must not draw it.
		{"Rocket | Out Of Feed Order", "2026-07-31T18:00:00Z", "Baikonur Cosmodrome",
			geo.Point{Lon: 63.564003, Lat: 45.996034}, false, ""},
		{"Falcon 9 Block 5 | Starlink Group 17-52", "2026-08-01T02:00:00Z", "Vandenberg SFB",
			geo.Point{Lon: -120.611, Lat: 34.632}, false, "LEO"},
		{"Rocket | Second Flight From One Pad", "2026-08-04T14:00:00Z", "Vandenberg SFB",
			geo.Point{Lon: -120.611, Lat: 34.632}, false, ""},
		{"Spectrum | Onward and Upward", "2026-08-06T00:00:00Z", "Andøya Spaceport",
			geo.Point{Lon: 15.5895, Lat: 69.1084}, true, "SSO"},
		{"Rocket | Unknown Precision", "2026-08-11T09:30:00Z", "Satish Dhawan Space Centre",
			geo.Point{Lon: 80.2304, Lat: 13.7199}, true, ""},
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
		if got.Orbit != w.orbit {
			t.Errorf("%s heads for %q, want %q", w.name, got.Orbit, w.orbit)
		}
		if i > 0 && got.At.Before(launches[i-1].At) {
			t.Errorf("%s is out of order", w.name)
		}
	}

	// Two of the five share a pad, and one pad is one dot.
	if pads := Pads(launches); len(pads) != 4 {
		t.Errorf("grouped into %d pads, want 4", len(pads))
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

// TestSeedCarriesADrawableLayer is what the committed feed is for: a cold cache
// with both endpoints refusing still leaves pads on the map.
func TestSeedCarriesADrawableLayer(t *testing.T) {
	taken, err := time.Parse(time.RFC3339, seedTaken)
	if err != nil {
		t.Fatalf("seedTaken: %v", err)
	}

	launches, err := launchesFrom(nil, errors.New("http 429"), taken.Add(time.Hour))
	if err != nil {
		t.Fatalf("the seed did not stand in: %v", err)
	}
	if len(launches) == 0 {
		t.Fatal("the seed parsed to no launches at all")
	}
	for _, l := range launches {
		if l.Position == (geo.Point{}) {
			t.Errorf("%q was seeded at the null island", l.Name)
		}
	}
	if pads := Pads(launches); len(pads) == 0 {
		t.Error("the seed grouped into no pads")
	}
}

// TestSeedExpires holds the seed to the cache's rule. A committed feed cannot
// refresh itself, so the only honest thing it can do is stop being believed.
func TestSeedExpires(t *testing.T) {
	taken, err := time.Parse(time.RFC3339, seedTaken)
	if err != nil {
		t.Fatalf("seedTaken: %v", err)
	}

	for _, tc := range []struct {
		name string
		age  time.Duration
		want bool
	}{
		{"inside the day these T-0s hold for", launchMaxAge - time.Hour, true},
		{"a day on, too many have slipped", launchMaxAge + time.Hour, false},
	} {
		_, err := launchSeed(taken.Add(tc.age))
		if got := err == nil; got != tc.want {
			t.Errorf("%s: seed served = %v, want %v (err %v)", tc.name, got, tc.want, err)
		}
	}
}

// TestOfflineDrawsNoSeed keeps the flag honest: -offline asks for the degraded
// map, and a seeded launch layer is exactly what would hide the degradation.
func TestOfflineDrawsNoSeed(t *testing.T) {
	if _, err := launchesFrom(nil, ErrOffline, feedNow); !errors.Is(err, ErrOffline) {
		t.Fatalf("got %v, want ErrOffline", err)
	}
}

// TestSeedIsNotReachedWhenAFeedArrived guards the order: a live body wins even
// on the day the seed would also parse.
func TestSeedIsNotReachedWhenAFeedArrived(t *testing.T) {
	launches, err := launchesFrom(readTestdata(t, "launches.json"), nil, feedNow)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(launches) != 5 {
		t.Fatalf("parsed %d launches, want the captured feed's 5: %v", len(launches), names(launches))
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

// TestLaunchesDropsFlightsThatAlreadyWent covers what the live feed happens not
// to contain today: a flight the map would otherwise still call the next one.
func TestLaunchesDropsFlightsThatAlreadyWent(t *testing.T) {
	for _, tc := range []struct {
		status string
		drawn  bool
		vague  bool
		flying bool
	}{
		{"Go", true, false, false},
		{"TBC", true, false, false},
		{"TBD", true, false, false},
		{"Hold", true, true, false},
		{"Success", false, false, false},
		{"Failure", false, false, false},
		{"Partial Failure", false, false, false},
		{"In Flight", true, false, true},
		{"Something New", true, false, false},
	} {
		body := feedWithStatus(tc.status)
		launches, err := parseLaunches(body, feedNow)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.status, err)
		}
		if drawn := len(launches) == 1; drawn != tc.drawn {
			t.Errorf("%s: drawn = %v, want %v", tc.status, drawn, tc.drawn)
			continue
		}
		if !tc.drawn {
			continue
		}
		if launches[0].Vague != tc.vague {
			t.Errorf("%s: vague = %v, want %v", tc.status, launches[0].Vague, tc.vague)
		}
		if launches[0].Flying != tc.flying {
			t.Errorf("%s: flying = %v, want %v", tc.status, launches[0].Flying, tc.flying)
		}
	}
}

// TestLaunchesDrawsARocketThatIsAirborne pins the one case a past T-0 is not a
// reason to drop the flight: the feed says it is climbing right now.
func TestLaunchesDrawsARocketThatIsAirborne(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status string
		net    time.Duration
		drawn  bool
	}{
		{"just lifted off", "In Flight", -2 * time.Minute, true},
		{"still inside the ascent window", "In Flight", -(ascentWindow - time.Minute), true},
		{"stale beyond the ascent window", "In Flight", -(ascentWindow + time.Minute), false},
		{"a past T-0 with no such status", "Go", -2 * time.Minute, false},
	} {
		body := feedAt(tc.status, feedNow.Add(tc.net))
		launches, err := parseLaunches(body, feedNow)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		if drawn := len(launches) == 1; drawn != tc.drawn {
			t.Errorf("%s: drawn = %v, want %v", tc.name, drawn, tc.drawn)
			continue
		}
		if tc.drawn && !launches[0].Flying {
			t.Errorf("%s: flying = false, want true", tc.name)
		}
	}
}

// feedWithStatus is one minute-precise flight, well inside the window, whose
// only interesting property is the status upstream put on it.
func feedWithStatus(status string) []byte {
	return feedAt(status, feedNow.Add(48*time.Hour))
}

func feedAt(status string, net time.Time) []byte {
	return fmt.Appendf(nil, `{"results":[{
		"name":"Rocket | Payload","net":%q,
		"net_precision":{"abbrev":"MIN"},"status":{"abbrev":%q},
		"launch_service_provider":{"abbrev":"ACME"},
		"pad":{"name":"LC-1","latitude":28.5,"longitude":-80.5,
		"location":{"name":"Cape Canaveral SFS, FL, USA"}}}]}`,
		net.Format(time.RFC3339), status)
}

func names(launches []Launch) []string {
	var out []string
	for _, l := range launches {
		out = append(out, l.Name)
	}
	return out
}
