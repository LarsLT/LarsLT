// Package sources fetches the live data the map draws. Every fetch degrades on
// its own: a source that fails drops its layer instead of failing the build.
package sources

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	userAgent  = "LarsLT-space-map (+https://github.com/LarsLT/LarsLT)"
	timeout    = 20 * time.Second
	retryPause = 3 * time.Second
	maxBody    = 8 << 20
)

// ErrOffline is what every source returns when the build is run with -offline.
var ErrOffline = fmt.Errorf("offline")

// Client is a small JSON getter with a timeout, one retry, and a copy of the
// last good response on disk to lean on when upstream rate-limits us.
type Client struct {
	HTTP     *http.Client
	CacheDir string
	Offline  bool
}

// New returns a Client. An empty cacheDir disables the fallback copy.
func New(cacheDir string, offline bool) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: timeout},
		CacheDir: cacheDir,
		Offline:  offline,
	}
}

// Fetch returns the body of url, or the cached copy if the request fails.
// Offline mode skips the cache too, so degradation is what actually gets tested.
func (c *Client) Fetch(ctx context.Context, url, cacheKey string) ([]byte, error) {
	if c.Offline {
		return nil, ErrOffline
	}

	body, err := c.get(ctx, url)
	if err == nil {
		c.store(cacheKey, body)
		return body, nil
	}

	cached, cacheErr := c.load(cacheKey)
	if cacheErr != nil {
		return nil, err
	}
	log.Printf("%s failed (%v), using the cached copy", cacheKey, err)
	return cached, nil
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := range 2 {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryPause):
			}
		}
		body, err := c.attempt(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
	}
	return nil, lastErr
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
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, maxBody))
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
	if err := os.MkdirAll(c.CacheDir, 0o755); err != nil {
		log.Printf("cache %s: %v", key, err)
		return
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		log.Printf("cache %s: %v", key, err)
	}
}

func (c *Client) load(key string) ([]byte, error) {
	path := c.cachePath(key)
	if path == "" {
		return nil, fmt.Errorf("no cache dir")
	}
	return os.ReadFile(path)
}
