package crawler

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
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
		MaxConnsPerHost:     20,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableCompression:  false,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: false},
		ForceAttemptHTTP2:   true,
	}

	return &Fetcher{
		client:     &http.Client{Transport: transport, Timeout: cfg.RequestTimeout},
		config:     cfg,
		hostDelays: make(map[string]time.Time),
	}
}

type FetchResult struct {
	Content     []byte
	StatusCode  int
	Headers     http.Header
	LoadTimeMs  int64
	ContentType string
	Error       error
}

func (f *Fetcher) Fetch(ctx context.Context, rawURL string) *FetchResult {
	result := &FetchResult{}

	// Normalize URL
	if !strings.HasPrefix(rawURL, "http") {
		rawURL = "https://" + rawURL
	}

	// Respect rate limits per host
	f.waitForHost(rawURL)

	// Retry loop with exponential backoff
	var lastErr error
	for attempt := 0; attempt <= f.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := f.config.RetryDelay * time.Duration(1<<uint(attempt-1))
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				result.Error = fmt.Errorf("context cancelled")
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
		default:
			reader = resp.Body
		}

		// Read with 10MB limit
		body, err := io.ReadAll(io.LimitReader(reader, 10*1024*1024))
		if reader != resp.Body {
			reader.Close()
		}
		resp.Body.Close()

		if err != nil {
			lastErr = err
			continue
		}

		result.Content = body
		result.Error = nil
		return result
	}

	result.Error = fmt.Errorf("failed after %d retries: %v", f.config.MaxRetries, lastErr)
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
