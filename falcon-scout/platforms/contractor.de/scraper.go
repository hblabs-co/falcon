package contractorde

import (
	"fmt"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"
	"hblabs.co/falcon/scout/platformkit"
)

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

// InspectResult holds the output of inspecting a detail page.
type InspectResult struct {
	HTML             string
	Project          *Project
	ProjectCandidate *ProjectCandidate
}

// scrapeListing fetches the single listing page and extracts all candidates.
// No pagination — contractor.de shows everything on one page.
func scrapeListing() (candidates []*ProjectCandidate, html string, cardsSeen int, err error) {
	c := newCollector()
	var scrapeErr error

	c.OnResponse(func(r *colly.Response) {
		html = string(r.Body)
	})

	c.OnHTML(".contentContainer div[data-category]", func(e *colly.HTMLElement) {
		cardsSeen++
		candidate := parseProjectCard(e)
		if candidate != nil {
			candidates = append(candidates, candidate)
		}
	})

	c.OnError(func(r *colly.Response, collyErr error) {
		scrapeErr = platformkit.ErrorFromStatus(r.StatusCode, r.Request.URL.String(), collyErr)
	})

	if visitErr := c.Visit(searchURL); visitErr != nil {
		err = visitErr
		return
	}
	if scrapeErr != nil {
		err = scrapeErr
		return
	}
	return
}

// parseProjectCard extracts a ProjectCandidate from a single div[data-category].
func parseProjectCard(e *colly.HTMLElement) *ProjectCandidate {
	// Title + URL from h4 > a
	href := e.ChildAttr("h4 a", "href")
	title := strings.TrimSpace(e.ChildText("h4 a"))
	if href == "" || title == "" {
		return nil
	}
	url := baseURL + href

	// Inner structure: .hubdb-card-inner has two child divs.
	// Left (30%): location, start date, duration — 3 paragraphs
	// Right (70%): project ID, description, requirements
	inner := e.DOM.Find(".hubdb-card-inner")
	leftDiv := inner.Children().First()
	rightDiv := inner.Children().Last()

	// Left column: 3 paragraphs by position
	var locationRaw, startDateRaw, duration string
	leftDiv.Find("p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		switch i {
		case 0:
			locationRaw = text
		case 1:
			startDateRaw = text
		case 2:
			duration = text
		}
	})

	// Right column: first p = "Projekt-ID XXXXX", second p = description
	var platformID, description string
	rightDiv.Find("p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		switch i {
		case 0:
			platformID = parseProjectID(text)
		case 1:
			description = text
		}
	})

	if platformID == "" {
		return nil
	}

	location, remote := parseLocation(locationRaw)
	startDate := parseStartDate(startDateRaw)

	// Requirements from <details> if present
	requirements := strings.TrimSpace(e.ChildText("details p"))

	return &ProjectCandidate{
		PlatformID:   platformID,
		URL:          url,
		Source:       Source,
		Title:        platformkit.NormalizeText(title),
		Location:     location,
		Remote:       remote,
		StartDate:    startDate,
		Duration:     platformkit.NormalizeText(duration),
		Description:  platformkit.NormalizeText(description),
		Requirements: platformkit.NormalizeText(requirements),
		ScrapedAt:    time.Now(),
	}
}

// Inspect visits a candidate's detail page to extract contact info.
// The listing page already has all project data; the detail page only adds
// the Ansprechpartner (contact person) name, phone, and email.
func Inspect(c *ProjectCandidate) (*InspectResult, error) {
	collector := newCollector()
	result := &InspectResult{ProjectCandidate: c}

	project := &Project{
		PlatformID:  c.PlatformID,
		URL:         c.URL,
		Title:       c.Title,
		Company:     Source,
		Description: buildDescription(c),
		Location:    c.Location,
		StartDate:   c.StartDate,
		Duration:    c.Duration,
		Remote:      c.Remote,
		Rate:        "Auf Anfrage",
		ScrapedAt:   time.Now(),
	}

	var scrapeErr error

	collector.OnResponse(func(r *colly.Response) {
		result.HTML = string(r.Body)
	})

	// Extract contact from .contactp
	collector.OnHTML(".contactp", func(e *colly.HTMLElement) {
		text := e.DOM.Find("p").Text()
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "Ihr Kontakt") {
				continue
			}
			if project.ContactName == "" && !strings.Contains(line, "@") && !strings.HasPrefix(line, "+") {
				project.ContactName = line
				continue
			}
		}
		// Phone from tel: link
		project.ContactPhone = strings.TrimSpace(e.ChildText(`a[href^="tel:"]`))
		// Email from mailto: link
		project.ContactEmail = strings.TrimSpace(e.ChildText(`a[href^="mailto:"]`))
	})

	collector.OnError(func(r *colly.Response, err error) {
		scrapeErr = platformkit.ErrorFromStatus(r.StatusCode, r.Request.URL.String(), err)
	})

	if err := collector.Visit(c.URL); err != nil {
		return nil, err
	}
	if scrapeErr != nil {
		return nil, scrapeErr
	}

	result.Project = project
	return result, nil
}

// buildDescription combines the candidate's description, requirements, and
// duration into a single text block for the Project.
func buildDescription(c *ProjectCandidate) string {
	var parts []string
	if c.Description != "" {
		parts = append(parts, c.Description)
	}
	if c.Duration != "" {
		parts = append(parts, fmt.Sprintf("Dauer: %s", c.Duration))
	}
	if c.Requirements != "" {
		parts = append(parts, fmt.Sprintf("Anforderungen: %s", c.Requirements))
	}
	return strings.Join(parts, "\n\n")
}
