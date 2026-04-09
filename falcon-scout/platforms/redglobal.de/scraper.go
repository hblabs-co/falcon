package redglobalde

import (
	"fmt"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"hblabs.co/falcon/common/helpers"
)

// collectCandidates scrapes listing pages starting at startPage.
// It returns all candidates found up to and including the last page that has results.
// shouldContinue is called after each page with the candidates found on that page;
// returning false stops pagination.
func collectCandidates(startPage int, shouldContinue func(page int, found []*ProjectCandidate) bool) ([]*ProjectCandidate, error) {
	var all []*ProjectCandidate
	page := startPage

	for {
		candidates, hasNext, err := scrapePage(page)
		if err != nil {
			return all, fmt.Errorf("scrape page %d: %w", page, err)
		}
		if len(candidates) == 0 {
			break
		}

		all = append(all, candidates...)

		if !shouldContinue(page, candidates) {
			break
		}
		if !hasNext {
			break
		}
		page++
	}
	return all, nil
}

// scrapePage fetches a single listing page and extracts candidates from c-card-job divs.
// It also returns whether a "next" pagination link exists.
func scrapePage(page int) ([]*ProjectCandidate, bool, error) {
	c := colly.NewCollector(
		colly.AllowedDomains("www.redglobal.de", "redglobal.de"),
	)

	var candidates []*ProjectCandidate
	hasNext := false
	var scrapeErr error

	c.OnHTML(".c-card-job", func(e *colly.HTMLElement) {
		candidate := parseJobCard(e)
		if candidate != nil {
			candidates = append(candidates, candidate)
		}
	})

	c.OnHTML(".c-pagination__next", func(_ *colly.HTMLElement) {
		hasNext = true
	})

	c.OnError(func(r *colly.Response, err error) {
		scrapeErr = fmt.Errorf("HTTP %d: %w", r.StatusCode, err)
	})

	url := fmt.Sprintf("%s&page=%d", searchURL, page)
	if err := c.Visit(url); err != nil {
		return nil, false, err
	}
	if scrapeErr != nil {
		return nil, false, scrapeErr
	}

	return candidates, hasNext, nil
}

// parseJobCard extracts a ProjectCandidate from a single c-card-job div.
// The href in c-card-job__actions > a has the form /jobs/job/<slug>/<platformID>.
func parseJobCard(e *colly.HTMLElement) *ProjectCandidate {
	href := e.ChildAttr(".c-card-job__actions a", "href")
	if href == "" {
		return nil
	}

	platformID, slug := parseJobHref(href)
	if platformID == "" {
		return nil
	}

	title := helpers.NormalizeText(e.ChildText(".c-card-job__title"))
	location := helpers.NormalizeText(e.ChildText(".c-list-specifications__item--location"))
	rate := helpers.NormalizeText(e.ChildText(".c-list-specifications__item--salary"))
	postedAt := helpers.NormalizeText(e.ChildText(".c-card-job__date"))

	return &ProjectCandidate{
		PlatformID: platformID,
		Slug:       slug,
		URL:        baseURL + href,
		Source:     Source,
		Title:      title,
		Location:   location,
		Rate:       rate,
		PostedAt:   postedAt,
		ScrapedAt:  time.Now(),
	}
}

// parseJobHref extracts platformID and slug from a path like /jobs/job/<slug>/<platformID>.
func parseJobHref(href string) (platformID, slug string) {
	// Trim trailing slash
	href = strings.TrimSuffix(href, "/")
	parts := strings.Split(href, "/")
	// Expected: ["", "jobs", "job", "<slug>", "<platformID>"]
	if len(parts) < 5 {
		return "", ""
	}
	return parts[len(parts)-1], parts[len(parts)-2]
}
