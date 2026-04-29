import re

with open('backend/internal/crawler/crawler.go', 'r') as f:
    text = f.read()

# Split out the sitemap fetcher
sitemap_part = text[text.find('// SitemapFetcher'):text.find('// HTTP API Server')]

# Write sitemap/discovery.go
discovery_code = """package sitemap

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

""" + sitemap_part.replace('SitemapFetcher', 'Fetcher')

with open('backend/internal/sitemap/discovery.go', 'w') as f:
    f.write(discovery_code)

# Clean up crawler.go
crawler_top = text[:text.find('// SEOResult mirrors')]
crawler_mid = text[text.find('// Crawler manages'):text.find('// SitemapFetcher')]

crawler_code = crawler_top.replace('package main', 'package crawler')
crawler_code = crawler_code.replace('"github.com/PuerkitoBio/goquery"\n)', '"github.com/PuerkitoBio/goquery"\n\t"seo-crawler/internal/models"\n)')
crawler_code += crawler_mid

# Replace struct names with models. equivalents
structs = ['SEOResult', 'ImageData', 'ImageStats', 'LinkData', 'LinkStats', 'CheckResult', 'Job']
for s in structs:
    crawler_code = re.sub(r'\b' + s + r'\b', f'models.{s}', crawler_code)

crawler_code = crawler_code.replace('CrawlerConfig', 'Config')
crawler_code = crawler_code.replace('NewCrawler', 'New')

with open('backend/internal/crawler/crawler.go', 'w') as f:
    f.write(crawler_code)

