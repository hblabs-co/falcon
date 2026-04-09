package redglobalde

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/modules/interfaces"
)

// Inspector fetches a detail page and extracts the JobPosting JSON-LD.
type Inspector struct {
	URL        string
	PlatformID string
	Current    int
	Total      int
	HTML       string // captured response body, available after Inspect()
}

func (i *Inspector) GetLogger() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{
		"source":  Source,
		"current": i.Current,
		"total":   i.Total,
		"url":     i.URL,
	})
}

// Inspect visits the detail page, finds the ld+json script with @type JobPosting,
// and returns a Project that implements interfaces.Project.
func (i *Inspector) Inspect() (interfaces.Project, error) {
	c := colly.NewCollector(
		colly.AllowedDomains("www.redglobal.de", "redglobal.de"),
	)

	var project *Project
	var scrapeErr error

	c.OnResponse(func(r *colly.Response) {
		i.HTML = string(r.Body)
	})

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
		project = jobPostingToProject(&ld, i.PlatformID)
	})

	c.OnError(func(r *colly.Response, err error) {
		scrapeErr = fmt.Errorf("HTTP %d: %w", r.StatusCode, err)
	})

	if err := c.Visit(i.URL); err != nil {
		return nil, err
	}
	if scrapeErr != nil {
		return nil, scrapeErr
	}
	if project == nil {
		return nil, fmt.Errorf("no JobPosting ld+json found on %s", i.URL)
	}

	return project, nil
}

// jobPostingToProject converts the parsed JSON-LD into a Project.
func jobPostingToProject(ld *jobPostingLD, platformID string) *Project {
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
		_, slug := parseJobHref(cleanURL)
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
