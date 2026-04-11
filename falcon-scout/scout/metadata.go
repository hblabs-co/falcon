package main

import (
	"context"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
)

// metadataRequestTimeout caps how long the metadata fetcher waits for any
// single file. These are tiny static documents — anything slower than this is
// almost certainly broken or hostile.
const metadataRequestTimeout = 10 * time.Second

// metadataMaxBodySize caps the body size we accept for any metadata file.
// 1 MB is far more than any sane robots.txt / security.txt / humans.txt; the
// limit is a defense against accidental memory bombs from misconfigured
// servers serving huge content under these paths.
const metadataMaxBodySize = 1 * 1024 * 1024

// fetchPlatformMetadata downloads the well-known metadata files for a platform
// rooted at baseURL and returns a map of file → content. Each file is optional:
// 404 / network errors / 5xx silently skip the file rather than failing the
// whole batch.
//
// Currently fetches:
//   - /robots.txt              → key "robots.txt"
//   - /.well-known/security.txt → key "security.txt"
//   - /humans.txt              → key "humans.txt"
//
// If robots.txt declares a "Sitemap:" directive, the URL is extracted and
// stored under "sitemap_url". The raw sitemap.xml itself is NOT fetched (it
// can be many megabytes); parse it in a dedicated job if you need its contents.
//
// All requests use a randomized realistic User-Agent rotated per request via
// colly's RandomUserAgent extension — same posture as the rest of the scout
// scrapers, even though metadata files are public and rarely subject to bot
// detection.
func fetchPlatformMetadata(ctx context.Context, baseURL string) map[string]string {
	files := make(map[string]string)

	if body := fetchOptional(ctx, baseURL+"/robots.txt"); body != "" {
		files["robots.txt"] = body
		if sitemap := extractSitemapURL(body); sitemap != "" {
			files["sitemap_url"] = sitemap
		}
	}

	if body := fetchOptional(ctx, baseURL+"/.well-known/security.txt"); body != "" {
		files["security.txt"] = body
	}

	if body := fetchOptional(ctx, baseURL+"/humans.txt"); body != "" {
		files["humans.txt"] = body
	}

	return files
}

// newMetadataCollector returns a colly collector configured for metadata
// fetching. Unlike the per-platform scrapers, it does NOT restrict
// AllowedDomains because the metadata loop targets whatever BaseURL each
// platform exposes, and we trust those URLs to be legitimate.
func newMetadataCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.MaxBodySize(metadataMaxBodySize),
	)
	c.SetRequestTimeout(metadataRequestTimeout)
	extensions.RandomUserAgent(c)
	return c
}

// fetchOptional downloads url and returns its body. Any failure (network, 4xx,
// 5xx) returns an empty string — callers decide whether the absence matters.
func fetchOptional(ctx context.Context, url string) string {
	if ctx.Err() != nil {
		return ""
	}
	c := newMetadataCollector()

	var body string
	c.OnResponse(func(r *colly.Response) {
		body = string(r.Body)
	})
	c.OnError(func(_ *colly.Response, _ error) {
		// Optional file — silently skip on any error.
	})
	_ = c.Visit(url)
	return body
}

// extractSitemapURL scans a robots.txt body for a "Sitemap:" directive and
// returns the first URL it finds. The directive is case-insensitive per the
// robots.txt spec and may appear anywhere in the file.
func extractSitemapURL(robotsTxt string) string {
	const prefix = "sitemap:"
	for _, line := range strings.Split(robotsTxt, "\n") {
		line = strings.TrimSpace(line)
		if len(line) > len(prefix) && strings.EqualFold(line[:len(prefix)], prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}
