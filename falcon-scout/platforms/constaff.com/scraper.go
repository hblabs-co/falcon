package constaffcom

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"hblabs.co/falcon/modules/platformkit"
)

var collectorCfg = platformkit.DefaultCollectorConfig(
	[]string{"www.constaff.com"},
	"*constaff.com*",
)

// detailPageURL builds the public detail page URL for a project.
func detailPageURL(projectID string) string {
	return fmt.Sprintf("%s/projektbeschreibung/?positionID=%s", baseURL, projectID)
}

// newDetailCollector creates a colly collector configured for polite
// scraping of constaff detail pages. The server is slow (4-10s per
// response) and aggressively rate-limits with 503, so we set a generous
// timeout and per-domain delay.
func newDetailCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.AllowedDomains(collectorCfg.AllowedDomains...),
	)
	c.UserAgent = platformkit.FalconUserAgent

	// Generous timeout — the server takes 4-10s to respond.
	c.WithTransport(&http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
	})

	// Rate limit: one request at a time with 5s between requests.
	// Prevents the 503 "Service Unavailable" that the server sends
	// when hit too fast in succession.
	_ = c.Limit(&colly.LimitRule{
		DomainGlob:  collectorCfg.DomainGlob,
		Delay:       collectorCfg.Delay,
		RandomDelay: collectorCfg.RandomDelay,
		Parallelism: collectorCfg.Parallelism,
	})

	return c
}

// scrapeContactFromDetailPage fetches the project detail HTML and
// extracts the contact info from the sidebar div. Used as fallback
// when the Ansprechpartner directory doesn't contain the User initials.
//
// Error handling follows the redglobal pattern: HTTP errors are wrapped
// via platformkit.ErrorFromStatus so the caller (and retry worker) can
// distinguish transient 5xx from permanent 4xx.
func scrapeContactFromDetailPage(c *colly.Collector, projectID string) (contactEntry, error) {
	var entry contactEntry
	var scrapeErr error
	found := false

	// Clone so each call gets its own visited-URL tracking while
	// sharing the parent's rate limiter and transport.
	cc := c.Clone()

	cc.OnHTML(".projectdetails-details-sidebar-contact", func(e *colly.HTMLElement) {
		found = true
		entry.Name = strings.TrimSpace(e.ChildText(".projectdetails-details-sidebar-contact-p-bold"))

		e.ForEach(".projectdetails-details-sidebar-contact-p", func(_ int, el *colly.HTMLElement) {
			text := strings.Join(strings.Fields(el.Text), " ") // collapse whitespace
			switch {
			case strings.HasPrefix(text, "Tel.:"):
				entry.Phone = strings.TrimSpace(strings.TrimPrefix(text, "Tel.:"))
			case strings.HasPrefix(text, "Mail:"):
				entry.Email = strings.TrimSpace(strings.TrimPrefix(text, "Mail:"))
			case entry.Position == "":
				entry.Position = text
			}
		})

		entry.Image = e.ChildAttr(".projectdetails-details-sidebar-picture", "src")
	})

	cc.OnError(func(r *colly.Response, err error) {
		scrapeErr = platformkit.ErrorFromStatus(r.StatusCode, r.Request.URL.String(), err)
	})

	url := detailPageURL(projectID)
	if err := cc.Visit(url); err != nil {
		return contactEntry{}, fmt.Errorf("visit %s: %w", url, err)
	}
	if scrapeErr != nil {
		return contactEntry{}, scrapeErr
	}
	if !found {
		return contactEntry{}, fmt.Errorf("no contact section found on %s", url)
	}
	return entry, nil
}
