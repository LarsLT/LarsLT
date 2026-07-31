package sources

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// goodTLE and goodKp are real answers, captured under cache/. Celestrak and GFZ
// both hand out prose instead when a runner is over quota.
const goodTLE = `ISS (ZARYA)
1 25544U 98067A   26211.15218593  .00008758  00000+0  16539-3 0  9990
2 25544  51.6319  87.4712 0007078 352.8836   7.2051 15.49259390578410
`

const goodKp = `{"Kp":[1.667,2.333,0.667,1.0],` +
	`"datetime":["2026-07-30T06:00:00Z","2026-07-30T09:00:00Z","2026-07-30T12:00:00Z","2026-07-30T15:00:00Z"],` +
	`"status":["pre","pre","pre","pre"]}`

// testClient never leaves the machine: every test points it at an httptest
// server, and the pause between attempts is what makes the retry table quick.
func testClient(cacheDir string) *Client {
	c := New(cacheDir, false)
	c.Pause = 0
	return c
}

func serve(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return srv
}

func seedCache(t *testing.T, dir, key string, body []byte, age time.Duration) string {
	t.Helper()
	path := filepath.Join(dir, key+".json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	if age > 0 {
		when := time.Now().Add(-age)
		if err := os.Chtimes(path, when, when); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestFetchDoesNotPoisonCacheOnBadBody(t *testing.T) {
	cases := []struct {
		name string
		req  Request
		good []byte
		bad  string
	}{
		{
			name: "launch library throttles with a 200",
			req:  Request{CacheKey: "launches", MaxAge: launchMaxAge, Valid: validLaunches},
			good: readTestdata(t, "launches.json"),
			bad:  `{"detail":"Request was throttled."}`,
		},
		{
			name: "an empty 200 is not a feed",
			req:  Request{CacheKey: "kp", Valid: validKp},
			good: []byte(goodKp),
			bad:  "",
		},
		{
			name: "celestrak answers over quota with prose",
			req:  Request{CacheKey: "iss", Valid: validTLE},
			good: []byte(goodTLE),
			bad:  "No GP data found\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := seedCache(t, dir, tc.req.CacheKey, tc.good, 0)
			srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte(tc.bad))
			})

			req := tc.req
			req.URL = srv.URL
			body, err := testClient(dir).Fetch(context.Background(), req)
			if err != nil {
				t.Fatalf("fetch fell through to an error instead of the cache: %v", err)
			}
			if !bytes.Equal(body, tc.good) {
				t.Errorf("caller got %d bytes, want the %d cached ones", len(body), len(tc.good))
			}

			onDisk, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(onDisk, tc.good) {
				t.Errorf("cache was overwritten with %q", onDisk)
			}
		})
	}
}

func TestFetchStoresAnAnswerThatParses(t *testing.T) {
	dir := t.TempDir()
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(goodTLE))
	})

	body, err := testClient(dir).Fetch(context.Background(),
		Request{URL: srv.URL, CacheKey: "iss", Valid: validTLE})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if string(body) != goodTLE {
		t.Errorf("got %q", body)
	}

	onDisk, err := os.ReadFile(filepath.Join(dir, "iss.json"))
	if err != nil {
		t.Fatalf("nothing was cached: %v", err)
	}
	if string(onDisk) != goodTLE {
		t.Errorf("cached %q", onDisk)
	}
}

// TestFetchRejectsBodiesNoParserWouldCatch uses a validator that accepts
// anything, so what fails here is the read itself rather than the source.
func TestFetchRejectsBodiesNoParserWouldCatch(t *testing.T) {
	cases := []struct {
		name string
		body []byte
	}{
		{"a 200 with nothing in it", nil},
		{"more than the reader will take whole", make([]byte, maxBody+1)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write(tc.body)
			})

			_, err := testClient(dir).Fetch(context.Background(), Request{
				URL: srv.URL, CacheKey: "iss", Valid: func([]byte) error { return nil },
			})
			if err == nil {
				t.Fatal("passed for a whole answer")
			}
			if _, err := os.Stat(filepath.Join(dir, "iss.json")); err == nil {
				t.Error("it reached the cache anyway")
			}
		})
	}
}

// TestMirrorAnswersWhenTheLiveEndpointWillNot covers the steady state on a
// hosted runner: the shared address is over quota before the build starts.
func TestMirrorAnswersWhenTheLiveEndpointWillNot(t *testing.T) {
	cases := []struct {
		name  string
		reply http.HandlerFunc
	}{
		{"over quota", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(http.StatusTooManyRequests)
		}},
		{"throttled behind a 200", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"detail":"Request was throttled."}`))
		}},
	}

	good := readTestdata(t, "launches.json")
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			live := serve(t, tc.reply)
			mirror := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.Write(good)
			})

			body, err := testClient(dir).Fetch(context.Background(), Request{
				URL: live.URL, Mirror: mirror.URL, CacheKey: "launches",
				MaxAge: launchMaxAge, Valid: validLaunches,
			})
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if !bytes.Equal(body, good) {
				t.Errorf("caller got %d bytes, want the mirror's %d", len(body), len(good))
			}

			onDisk, err := os.ReadFile(filepath.Join(dir, "launches.json"))
			if err != nil {
				t.Fatalf("the mirror's answer was not cached: %v", err)
			}
			if !bytes.Equal(onDisk, good) {
				t.Errorf("cached %d bytes, want %d", len(onDisk), len(good))
			}
		})
	}
}

func TestMirrorIsLeftAloneWhenTheLiveEndpointAnswers(t *testing.T) {
	good := readTestdata(t, "launches.json")
	live := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(good)
	})
	mirror := serve(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("the mirror was asked for a feed the live endpoint had served")
	})

	if _, err := testClient(t.TempDir()).Fetch(context.Background(), Request{
		URL: live.URL, Mirror: mirror.URL, CacheKey: "launches",
		MaxAge: launchMaxAge, Valid: validLaunches,
	}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
}

// TestMirrorBeatsTheCache pins the order: a mirror is a live answer, and the
// cached copy is however old the last outage left it.
func TestMirrorBeatsTheCache(t *testing.T) {
	dir := t.TempDir()
	seedCache(t, dir, "launches", []byte(`{"results":[{"name":"stale"}]}`), 12*time.Hour)
	good := readTestdata(t, "launches.json")

	live := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "over quota", http.StatusTooManyRequests)
	})
	mirror := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write(good)
	})

	body, err := testClient(dir).Fetch(context.Background(), Request{
		URL: live.URL, Mirror: mirror.URL, CacheKey: "launches",
		MaxAge: launchMaxAge, Valid: validLaunches,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !bytes.Equal(body, good) {
		t.Error("the cached copy won over a mirror that answered")
	}
}

// TestCacheStillCatchesBothEndpointsDown is the layer under the mirror: the
// throttle that reaches one endpoint usually reaches the other.
func TestCacheStillCatchesBothEndpointsDown(t *testing.T) {
	dir := t.TempDir()
	good := readTestdata(t, "launches.json")
	seedCache(t, dir, "launches", good, time.Hour)

	down := func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "over quota", http.StatusTooManyRequests)
	}
	live, mirror := serve(t, down), serve(t, down)

	body, err := testClient(dir).Fetch(context.Background(), Request{
		URL: live.URL, Mirror: mirror.URL, CacheKey: "launches",
		MaxAge: launchMaxAge, Valid: validLaunches,
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !bytes.Equal(body, good) {
		t.Error("the cached copy was not served")
	}
}

func TestOfflineFetchesNeitherEndpoint(t *testing.T) {
	reached := func(w http.ResponseWriter, r *http.Request) {
		t.Error("offline mode reached the network")
	}
	live, mirror := serve(t, reached), serve(t, reached)

	c := testClient(t.TempDir())
	c.Offline = true
	if _, err := c.Fetch(context.Background(), Request{
		URL: live.URL, Mirror: mirror.URL, CacheKey: "launches", Valid: validLaunches,
	}); err != ErrOffline {
		t.Fatalf("got %v, want ErrOffline", err)
	}
}

func TestStaleCacheIsRejected(t *testing.T) {
	good := readTestdata(t, "launches.json")
	cases := []struct {
		name string
		age  time.Duration
		want bool
	}{
		{"inside the day the feed stays useful for", 23 * time.Hour, true},
		{"a T-0 a day old has slipped", 25 * time.Hour, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			seedCache(t, dir, "launches", good, tc.age)
			srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "down", http.StatusServiceUnavailable)
			})

			_, err := testClient(dir).Fetch(context.Background(), Request{
				URL: srv.URL, CacheKey: "launches", MaxAge: launchMaxAge, Valid: validLaunches,
			})
			if got := err == nil; got != tc.want {
				t.Fatalf("served from cache = %v, want %v (err %v)", got, tc.want, err)
			}
		})
	}
}

// TestElementCacheOutlivesItsFile covers the sources that date their own data:
// the file's age says nothing, and the TLE epoch is what ISS checks instead.
func TestElementCacheOutlivesItsFile(t *testing.T) {
	dir := t.TempDir()
	seedCache(t, dir, "iss", []byte(goodTLE), 30*24*time.Hour)
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "over quota", http.StatusTooManyRequests)
	})

	body, err := testClient(dir).Fetch(context.Background(),
		Request{URL: srv.URL, CacheKey: "iss", Valid: validTLE})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if string(body) != goodTLE {
		t.Errorf("got %q", body)
	}
}

func TestRetryPolicy(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		retryAfter string
		want       int
	}{
		{"a 404 says the same thing twice", http.StatusNotFound, "", 1},
		{"a 400 is not going to improve", http.StatusBadRequest, "", 1},
		{"a throttle we cannot wait out is quota spent", http.StatusTooManyRequests, "120", 1},
		{"a throttle that lifts in time is worth honouring", http.StatusTooManyRequests, "1", 2},
		{"a throttle without a header gets one more go", http.StatusTooManyRequests, "", 2},
		{"a 500 might be a blip", http.StatusInternalServerError, "", 2},
		{"a 408 might be a blip", http.StatusRequestTimeout, "", 2},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hits := 0
			srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
				hits++
				if tc.retryAfter != "" {
					w.Header().Set("Retry-After", tc.retryAfter)
				}
				w.WriteHeader(tc.status)
			})

			_, err := testClient("").Fetch(context.Background(),
				Request{URL: srv.URL, Valid: func([]byte) error { return nil }})
			if err == nil {
				t.Fatal("a refused request came back without an error")
			}
			if hits != tc.want {
				t.Errorf("upstream saw %d requests, want %d", hits, tc.want)
			}
		})
	}
}

func TestRetryIsSkippedWithoutBudget(t *testing.T) {
	hits := 0
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "down", http.StatusServiceUnavailable)
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := testClient("").Fetch(ctx,
		Request{URL: srv.URL, Valid: func([]byte) error { return nil }}); err == nil {
		t.Fatal("expected an error")
	}
	if hits != 1 {
		t.Errorf("upstream saw %d requests, want 1: no room left for a second attempt", hits)
	}
}

func TestOfflineFetchesNothing(t *testing.T) {
	dir := t.TempDir()
	seedCache(t, dir, "iss", []byte(goodTLE), 0)
	srv := serve(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("offline mode reached the network")
	})

	c := testClient(dir)
	c.Offline = true
	if _, err := c.Fetch(context.Background(),
		Request{URL: srv.URL, CacheKey: "iss", Valid: validTLE}); err != ErrOffline {
		t.Fatalf("got %v, want ErrOffline", err)
	}
}

func TestRetryAfterReadsBothForms(t *testing.T) {
	if got := retryAfter("12"); got != 12*time.Second {
		t.Errorf("seconds form: got %v", got)
	}
	if got := retryAfter(time.Now().Add(30 * time.Second).UTC().Format(http.TimeFormat)); got < 25*time.Second {
		t.Errorf("date form: got %v", got)
	}
	if got := retryAfter(""); got != 0 {
		t.Errorf("missing header: got %v", got)
	}
	if got := retryAfter("0"); got != 0 {
		t.Errorf("zero: got %v", got)
	}
}
