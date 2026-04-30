package crawler

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type Fetcher struct {
	client     *http.Client
	config     Config
	hostDelays map[string]time.Time
	delayMu    sync.RWMutex
}

func NewFetcher(cfg Config) *Fetcher {
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
		ForceAttemptHTTP2:   true,
		TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	}

	return &Fetcher{
		client: &http.Client{
			Transport: transport,
			Timeout:   cfg.RequestTimeout,
		},
		config:     cfg,
		hostDelays: make(map[string]time.Time),
	}
}

// Fetch returns *FetchResult (single return value)
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) *FetchResult {
	f.waitForHost(rawURL)

	result := &FetchResult{}

	var lastErr error
	for attempt := 0; attempt <= f.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := f.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				result.Error = ctx.Err()
				return result
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		req.Header.Set("User-Agent", f.config.UserAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "en-US,en;q=0.5")
		req.Header.Set("Accept-Encoding", "gzip")

		start := time.Now()
		resp, err := f.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		result.LoadTimeMs = time.Since(start).Milliseconds()
		result.StatusCode = resp.StatusCode
		result.Headers = resp.Header
		result.ContentType = resp.Header.Get("Content-Type")

		// Handle gzip
		var reader io.ReadCloser
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			reader, err = gzip.NewReader(resp.Body)
			if err != nil {
				resp.Body.Close()
				lastErr = err
				continue
			}
			defer reader.Close()
		default:
			reader = resp.Body
		}

		// Read with 10MB limit
		body, err := io.ReadAll(io.LimitReader(reader, 10*1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		result.Content = body
		result.Error = nil
		return result // Success
	}

	result.Error = lastErr
	return result
}

func (f *Fetcher) waitForHost(rawURL string) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	host := u.Host

	f.delayMu.Lock()
	defer f.delayMu.Unlock()

	if lastTime, exists := f.hostDelays[host]; exists {
		elapsed := time.Since(lastTime)
		if elapsed < f.config.PoliteDelay {
			time.Sleep(f.config.PoliteDelay - elapsed)
		}
	}
	f.hostDelays[host] = time.Now()
}
