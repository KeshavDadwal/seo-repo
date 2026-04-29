// backend/internal/crawler/crawler.go
package crawler

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"

	"seo-crawler/internal/models"
)

type Crawler struct {
	config    Config
	fetcher   *Fetcher
	parser    *Parser
	scorer    *Scorer
	semaphore chan struct{}
	progress  int32
}

func New(cfg Config) *Crawler {
	return &Crawler{
		config:    cfg,
		fetcher:   NewFetcher(cfg),
		parser:    NewParser(),
		scorer:    NewScorer(),
		semaphore: make(chan struct{}, cfg.MaxWorkers),
	}
}

func (c *Crawler) AnalyzePages(ctx context.Context, urls []string) []models.SEOResult {
	if len(urls) > c.config.MaxPages {
		urls = urls[:c.config.MaxPages]
	}

	var wg sync.WaitGroup
	resultCh := make(chan models.SEOResult, len(urls))

	for _, rawURL := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()

			select {
			case c.semaphore <- struct{}{}:
				defer func() { <-c.semaphore }()
			case <-ctx.Done():
				return
			}

			result := c.analyzeSingle(ctx, u)
			atomic.AddInt32(&c.progress, 1)
			resultCh <- result
		}(rawURL)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []models.SEOResult
	for r := range resultCh {
		results = append(results, r)
	}

	return results
}

func (c *Crawler) analyzeSingle(ctx context.Context, rawURL string) models.SEOResult {
	result := models.SEOResult{
		URL: rawURL,
	}

	// Normalize URL
	if !strings.HasPrefix(rawURL, "http") {
		rawURL = "https://" + rawURL
		result.HasHTTPS = true
	}

	fetchResult := c.fetcher.Fetch(ctx, rawURL)

	if fetchResult.Error != nil {
		result.Error = fetchResult.Error.Error()
		if fetchResult.StatusCode > 0 {
			result.StatusCode = fetchResult.StatusCode
		}
		return result
	}

	result.StatusCode = fetchResult.StatusCode
	result.LoadTimeMs = fetchResult.LoadTimeMs
	// result.PageSizeBytes = fetchResult.PageSizeBytes
	result.ContentType = fetchResult.ContentType
	result.XFrameOptions = fetchResult.Headers.Get("X-Frame-Options")
	result.ContentSecurity = fetchResult.Headers.Get("Content-Security-Policy")

	html := string(fetchResult.Content)
	parsed := c.parser.Parse(html, rawURL, fetchResult)

	// Copy parsed data
	result.Title = parsed.Title
	result.TitleLength = parsed.TitleLength
	result.MetaDescription = parsed.MetaDescription
	result.MetaDescriptionLength = parsed.MetaDescriptionLength
	result.MetaKeywords = parsed.MetaKeywords
	result.MetaViewport = parsed.MetaViewport
	result.Canonical = parsed.Canonical
	result.RobotsMeta = parsed.RobotsMeta
	result.Headings = parsed.Headings
	result.Images = parsed.Images
	result.ImageStats = parsed.ImageStats
	result.Links = parsed.Links
	result.LinkStats = parsed.LinkStats
	result.WordCount = parsed.WordCount
	result.OGTags = parsed.OGTags
	result.SchemaTypes = parsed.SchemaTypes
	result.HasHTTPS = parsed.HasHTTPS

	c.scorer.Calculate(&result)
	return result
}

func (c *Crawler) Progress() int32 {
	return atomic.LoadInt32(&c.progress)
}
