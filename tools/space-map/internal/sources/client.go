// Package sources fetches the live data the map draws. Every fetch degrades on
// its own: a source that fails drops its layer instead of failing the build.
package sources

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const (
	userAgent  = "LarsLT-space-map (+https://github.com/LarsLT/LarsLT)"
	timeout    = 20 * time.Second
	retryPause = 3 * time.Second
	maxBody    = 8 << 20
)

// maxRetryWait is the longest Retry-After worth honouring: the map is redrawn
// every half hour, so waiting out a longer throttle spends the run for nothing.
const maxRetryWait = 10 * time.Second

// ErrOffline is what every source returns when the build is run with -offline.
var ErrOffline = fmt.Errorf("offline")

// Request is one source's fetch: where it comes from, what its cached copy is
// called, how stale that copy may get, and how to tell data from an error page.
type Request struct {
	URL      string
	CacheKey string
	// MaxAge is zero for a source whose data carries its own timestamp, since
	// that says more about freshness than the file's date does.
	MaxAge time.Duration
	// Valid parses the body. Upstream answers an over-quota request with 200 and
	// a sentence, so a body is only data once something has read it.
	Valid func(body []byte) error
}

// Client is a small JSON getter with a timeout, one retry, and a copy of the
// last good response on disk to lean on when upstream rate-limits us.
type Client struct {
	HTTP     *http.Client
	CacheDir string
	Offline  bool
	Pause    time.Duration // wait between the two attempts
}

// New returns a Client. An empty cacheDir disables the fallback copy.
func New(cacheDir string, offline bool) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: timeout},
		CacheDir: cacheDir,
		Offline:  offline,
		Pause:    retryPause,
	}
}

// Fetch returns the body of req.URL, or the cached copy when the live answer
// fails or does not parse. Only a body Valid has read ever reaches the cache.
func (c *Client) Fetch(ctx context.Context, req Request) ([]byte, error) {
	if c.Offline {
		// Skipping the cache too is what makes the degraded map testable.
		return nil, ErrOffline
	}

	body, err := c.get(ctx, req.URL)
	if err == nil {
		if err = req.Valid(body); err == nil {
			c.store(req.CacheKey, body)
			return body, nil
		}
		err = fmt.Errorf("live answer rejected: %w", err)
	}

	cached, cacheErr := c.load(req.CacheKey, req.MaxAge)
	if cacheErr != nil {
		return nil, err
	}
	if validErr := req.Valid(cached); validErr != nil {
		log.Printf("%s cached copy is unusable: %v", req.CacheKey, validErr)
		return nil, err
	}
	log.Printf("%s failed (%v), using the cached copy", req.CacheKey, err)
	return cached, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	body, err := c.attempt(ctx, url)
	if err == nil {
		return body, nil
	}
	pause, again := c.retryIn(err)
	if !again {
		return nil, err
	}
	// A retry that cannot finish inside the run's budget is worse than none: it
	// spends the time the remaining layers need to reach their own caches.
	if deadline, set := ctx.Deadline(); set && time.Until(deadline) < pause+timeout {
		return nil, err
	}
	select {
	case <-ctx.Done():
		return nil, err
	case <-time.After(pause):
	}
	return c.attempt(ctx, url)
}

// retryIn says how long to hold off before the second attempt, and whether the
// answer could differ at all: a 404 says the same thing twice.
func (c *Client) retryIn(err error) (time.Duration, bool) {
	var status *statusErr
	if !errors.As(err, &status) {
		return c.Pause, true // a dropped connection is worth one more go
	}
	switch {
	case status.code >= 500, status.code == http.StatusRequestTimeout:
		return c.Pause, true
	case status.code == http.StatusTooManyRequests:
		if status.after > maxRetryWait {
			return 0, false
		}
		return max(status.after, c.Pause), true
	}
	return 0, false
}

func (c *Client) attempt(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &statusErr{code: resp.StatusCode, after: retryAfter(resp.Header.Get("Retry-After"))}
	}

	// One byte past the limit, so an oversized feed reads as an error rather than
	// as a truncated one that happens to parse.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	switch {
	case len(body) == 0:
		return nil, fmt.Errorf("empty body")
	case len(body) > maxBody:
		return nil, fmt.Errorf("body over the %d byte limit", maxBody)
	}
	return body, nil
}

// statusErr is a refused answer, kept whole because the retry policy turns on
// the code and on Retry-After.
type statusErr struct {
	code  int
	after time.Duration
}

func (e *statusErr) Error() string {
	if e.after > 0 {
		return fmt.Sprintf("http %d, retry after %s", e.code, e.after)
	}
	return fmt.Sprintf("http %d", e.code)
}

func retryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if secs, err := strconv.Atoi(header); err == nil {
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(header); err == nil {
		return time.Until(when)
	}
	return 0
}

func (c *Client) cachePath(key string) string {
	if c.CacheDir == "" {
		return ""
	}
	return filepath.Join(c.CacheDir, key+".json")
}

func (c *Client) store(key string, body []byte) {
	path := c.cachePath(key)
	if path == "" {
		return
	}
	if err := writeAtomic(path, body); err != nil {
		log.Printf("cache %s: %v", key, err)
	}
}

// writeAtomic renames a finished file into place, so a run killed mid-write
// leaves the last good copy whole instead of half a feed.
func writeAtomic(path string, body []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (c *Client) load(key string, maxAge time.Duration) ([]byte, error) {
	path := c.cachePath(key)
	if path == "" {
		return nil, fmt.Errorf("no cache dir")
	}
	if maxAge > 0 {
		// The workflow's cache rolls forward on every run, so a copy can outlive
		// the thing it describes by as long as upstream stays down.
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if age := time.Since(info.ModTime()); age > maxAge {
			return nil, fmt.Errorf("cached copy is %s old", age.Round(time.Minute))
		}
	}
	return os.ReadFile(path)
}
