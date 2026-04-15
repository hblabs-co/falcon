package redglobalde

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	"hblabs.co/falcon/modules/platformkit"
)

// newCollector returns a colly collector pre-configured for redglobal scraping:
//   - Allowed domains restricted to redglobal.de
//   - Random realistic User-Agent rotated per request (extensions.RandomUserAgent)
//   - Rate limit: 1 request at a time per domain, with 2s base delay + 0–2s jitter
//
// Both scrapePage and Inspect use this so listing and detail requests share the
// same anti-bot posture and rate budget.
var collectorCfg = platformkit.DefaultCollectorConfig(
	[]string{hostWWW, hostBare}, hostGlob,
)

func newCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains(collectorCfg.AllowedDomains...),
	)
	extensions.RandomUserAgent(c)
	_ = c.Limit(&colly.LimitRule{
		DomainGlob:  collectorCfg.DomainGlob,
		Parallelism: collectorCfg.Parallelism,
		Delay:       collectorCfg.Delay,
		RandomDelay: collectorCfg.RandomDelay,
	})
	return c
}

type Scraper struct {
	Url              string
	PlatformID       string
	Current          int
	Total            int
	HTML             string // captured response body, available after Inspect() regardless of outcome
	ProjectCandidate *ProjectCandidate
}

func NewScraper() *Scraper {
	return &Scraper{}
}

func NewCandidateScraper(c *ProjectCandidate) *Scraper {
	return &Scraper{
		Url:              c.URL,
		PlatformID:       c.PlatformID,
		Current:          c.Current,
		Total:            c.Total,
		ProjectCandidate: c,
	}
}

// pageHandlerFn is called after each page is scraped.
// It receives the page number and the raw candidates from that page.
// It returns the filtered candidates to keep and whether to continue to the next page.
type pageHandlerFn func(page int, found []*ProjectCandidate) (filtered []*ProjectCandidate, cont bool)

// scrapeLoop scrapes listing pages starting at startPage.
// The handler filters each page's candidates and decides whether to continue.
//
// If the FIRST page returns zero candidates, scrapeLoop returns a typed
// platformkit.ErrEmptyListing carrying the captured HTML so the caller can
// emit a categorical error: an empty page 1 on a site that's normally always
// populated almost always means the markup changed and the selectors are no
// longer matching anything. Empty pages on later pages are still treated as
// "end of listing" and break out of the loop normally.
func (s *Scraper) scrapeLoop(startPage int, handler pageHandlerFn) ([]*ProjectCandidate, error) {
	var all []*ProjectCandidate
	page := startPage

	for {
		candidates, hasNext, html, cardsSeen, err := s.scrapePage(page)
		if err != nil {
			return all, fmt.Errorf("scrape page %d: %w", page, err)
		}
		if len(candidates) == 0 {
			if page == startPage {
				return all, &platformkit.ErrEmptyListing{Page: page, HTML: html, CardsSeen: cardsSeen}
			}
			break
		}

		filtered, cont := handler(page, candidates)
		all = append(all, filtered...)

		if !cont || !hasNext {
			break
		}
		page++
	}
	return all, nil
}

// scrapePage fetches a single listing page and extracts candidates from c-card-job divs.
// Returns the candidates, whether a "next" pagination link exists, the raw
// HTML body (so callers can attach it to a categorical error if the page is
// unexpectedly empty), and any HTTP error.
func (s *Scraper) scrapePage(page int) (candidates []*ProjectCandidate, hasNext bool, html string, cardsSeen int, err error) {
	c := newCollector()
	var scrapeErr error

	c.OnResponse(func(r *colly.Response) {
		html = string(r.Body)
	})

	c.OnHTML(".c-card-job", func(e *colly.HTMLElement) {
		cardsSeen++
		candidate := s.parseJobCard(e)
		if candidate != nil {
			candidates = append(candidates, candidate)
		}
	})

	c.OnHTML(".c-pagination__next", func(_ *colly.HTMLElement) {
		hasNext = true
	})

	c.OnError(func(r *colly.Response, err error) {
		scrapeErr = platformkit.ErrorFromStatus(r.StatusCode, r.Request.URL.String(), err)
	})

	url := fmt.Sprintf("%s&page=%d", searchURL, page)
	if visitErr := c.Visit(url); visitErr != nil {
		err = visitErr
		return
	}
	if scrapeErr != nil {
		err = scrapeErr
		return
	}
	return
}

// parseJobCard extracts a ProjectCandidate from a single c-card-job div.
// The href in c-card-job__actions > a has the form /jobs/job/<slug>/<platformID>.
func (s *Scraper) parseJobCard(e *colly.HTMLElement) *ProjectCandidate {
	href := e.ChildAttr(".c-card-job__actions a", "href")
	if href == "" {
		return nil
	}

	platformID, slug := s.parseJobHref(href)
	if platformID == "" {
		return nil
	}

	title := platformkit.NormalizeText(e.ChildText(".c-card-job__title"))
	location := platformkit.NormalizeText(e.ChildText(".c-list-specifications__item--location"))
	rate := platformkit.NormalizeText(e.ChildText(".c-list-specifications__item--salary"))
	postedAt := parseListingDate(platformkit.NormalizeText(e.ChildText(".c-card-job__date")))

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

// InspectResult holds the output of inspecting a detail page.
type InspectResult struct {
	HTML             string
	Project          *Project
	ProjectCandidate *ProjectCandidate
}

// Inspect visits the detail page, finds the ld+json script with @type JobPosting,
// and returns a Project that implements interfaces.Project.
func (s *Scraper) Inspect() (*InspectResult, error) {
	c := newCollector()

	result := &InspectResult{}
	var project *Project
	var scrapeErr error

	c.OnResponse(func(r *colly.Response) {
		result.HTML = string(r.Body)
	})

	var referenceID string

	c.OnHTML(`script[type="application/ld+json"]`, func(e *colly.HTMLElement) {
		if project != nil {
			return // already found
		}
		raw := strings.TrimSpace(e.Text)
		var ld jobPostingLD
		if err := json.Unmarshal([]byte(raw), &ld); err != nil {
			return
		}
		if ld.Type != "JobPosting" {
			return
		}
		project = s.jobPostingToProject(&ld, s.PlatformID)
	})

	// Extract reference ID from <p><strong>Reference</strong><br />CR/...</p>.
	// Robust to label translation (English/German) and to extra punctuation.
	c.OnHTML("p:has(strong)", func(e *colly.HTMLElement) {
		if referenceID != "" {
			return
		}
		label := normalizeReferenceLabel(e.ChildText("strong"))
		if !isReferenceLabel(label) {
			return
		}
		// Extract the value by cloning the DOM, removing the <strong> child,
		// and reading what's left. This avoids the fragile TrimPrefix on a string
		// that may or may not have the exact label as prefix.
		referenceID = strings.TrimSpace(e.DOM.Clone().Find("strong").Remove().End().Text())
	})

	c.OnError(func(r *colly.Response, err error) {
		scrapeErr = platformkit.ErrorFromStatus(r.StatusCode, r.Request.URL.String(), err)
	})

	if err := c.Visit(s.Url); err != nil {
		return nil, err
	}
	if scrapeErr != nil {
		return nil, scrapeErr
	}
	if project == nil {
		return nil, fmt.Errorf("no JobPosting ld+json found on %s", s.Url)
	}

	project.ReferenceID = referenceID
	project.Rate = "Negotiable"

	// Use the candidate's listing date as the canonical update timestamp.
	// The listing date is what the filter compares against on the next poll,
	// so persisting the same source here keeps filter and save in agreement
	// even if the JSON-LD datePosted diverges from the listing date.
	if s.ProjectCandidate != nil && s.ProjectCandidate.PostedAt != "" {
		project.DatePosted = s.ProjectCandidate.PostedAt
	}

	result.Project = project
	result.ProjectCandidate = s.ProjectCandidate
	return result, nil
}

// jobPostingToProject converts the parsed JSON-LD into a Project.
func (s *Scraper) jobPostingToProject(ld *jobPostingLD, platformID string) *Project {
	p := &Project{
		PlatformID:  platformID,
		URL:         strings.ReplaceAll(ld.URL, `\/`, `/`),
		Title:       ld.Title,
		Company:     ld.HiringOrganization,
		Description: strings.TrimSpace(ld.Description),
		Industry:    ld.Industry,
		DatePosted:  ld.DatePosted,
		Remote:      strings.EqualFold(ld.JobLocationType, "TELECOMMUTE"),
		ScrapedAt:   time.Now(),
	}

	// Extract slug from URL
	if ld.URL != "" {
		cleanURL := strings.ReplaceAll(ld.URL, `\/`, `/`)
		_, slug := s.parseJobHref(cleanURL)
		p.Slug = slug
	}

	// Parse end date from validThrough (e.g. "2026-04-18T00:00:00+01:00")
	if ld.ValidThrough != "" {
		if t, err := time.Parse(time.RFC3339, ld.ValidThrough); err == nil {
			p.EndDate = t.Format("2006-01-02")
		} else {
			p.EndDate = ld.ValidThrough
		}
	}

	// Location from jobLocation.address
	if ld.JobLocation != nil && ld.JobLocation.Address != nil {
		p.Location = ld.JobLocation.Address.AddressLocality
	}

	return p
}
