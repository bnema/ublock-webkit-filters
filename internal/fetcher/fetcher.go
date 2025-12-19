package fetcher

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bnema/ublock-webkit-filters/internal/models"
)

// Fetcher downloads filter lists
type Fetcher struct {
	client  *http.Client
	retries int
}

// New creates a new fetcher from config
func New(cfg models.HTTPConfig) *Fetcher {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	retries := cfg.Retries
	if retries == 0 {
		retries = 3
	}

	return &Fetcher{
		client: &http.Client{
			Timeout: timeout,
		},
		retries: retries,
	}
}

// Fetch downloads content from a URL with retries
func (f *Fetcher) Fetch(ctx context.Context, url string) ([]byte, error) {
	var lastErr error

	for i := 0; i < f.retries; i++ {
		if i > 0 {
			// Exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(i) * time.Second):
			}
		}

		data, err := f.doFetch(ctx, url)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d retries: %w", f.retries, lastErr)
}

func (f *Fetcher) doFetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "ublock-webkit-filters/1.0")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}
