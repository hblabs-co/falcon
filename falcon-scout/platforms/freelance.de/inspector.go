package freelancede

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/gocolly/colly/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/interfaces"
)

// Inspector implements Inspector for freelance.de. Cookies are read
// from environment variables:
//
//	FREELANCE_DE_COOKIE      → cookie name "freelance"
//	FREELANCE_DE_USER_COOKIE → cookie name "freelance_user"
//	FREELANCE_DE_SSO_COOKIE            → cookie name "sso"
type Inspector struct {
	Url        string
	PlatformID string
	Current    int
	Total      int
	HTML       string // captured response body, available after Inspect() regardless of outcome
}

func (pi *Inspector) GetLogger() *logrus.Entry {
	return getLogger().WithFields(logrus.Fields{"current": pi.Current, "total": pi.Total, "url": pi.Url})
}

// Inspect visits the configured URL, extracts the project data, and returns it.
// If the session is expired it attempts a re-login once and retries automatically.
func (pi *Inspector) Inspect() (interfaces.Project, error) {
	project, err := pi.inspect()
	if err == ErrSessionExpired {
		pi.GetLogger().Warn("freelance.de: session expired, attempting re-login")
		if loginErr := getSession().Login(); loginErr != nil {
			return nil, err
		}
		return pi.inspect()
	}
	return project, err
}

// inspect performs a single scrape attempt using the current session cookies.
func (pi *Inspector) inspect() (*Project, error) {
	c := colly.NewCollector(
		colly.AllowedDomains("www.freelance.de", "freelance.de"),
	)

	if err := c.SetCookies(baseUrl, getSession().Cookies()); err != nil {
		pi.GetLogger().Warnf("warning: could not set cookies: %v", err)
	}

	var project Project
	project.URL = pi.Url

	var scrapeErr error
	registerHandlers(c, &project)

	c.OnResponse(func(r *colly.Response) {
		pi.HTML = string(r.Body)
		if bytes.Contains(r.Body, []byte("für EXPERT-Mitglieder sichtbar")) {
			scrapeErr = ErrSessionExpired
		}
	})

	c.OnScraped(func(_ *colly.Response) {
		if _, err := json.Marshal(project); err != nil {
			pi.GetLogger().Errorf("error marshaling project: %v", err)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		pi.GetLogger().Errorf("request error: status=%d err=%v", r.StatusCode, err)
		if r.StatusCode >= 500 {
			scrapeErr = &ErrServerError{StatusCode: r.StatusCode, URL: pi.Url}
		}
	})

	if err := c.Visit(pi.Url); err != nil {
		return nil, err
	}
	if scrapeErr != nil {
		return nil, scrapeErr
	}

	return &project, nil
}

// registerHandlers attaches all HTML parsing callbacks to c, populating project
// as the page is scraped. It does not handle persistence or visiting.
func registerHandlers(c *colly.Collector, project *Project) {
	// --- Direct client / ANUE badges -----------------------------------------
	// Both badges live as <span> inside <p class="company-name">.
	// Direktkunden-Projekt uses badge-light; Arbeitnehmerüberlassung uses bg-black.
	c.OnHTML("#project_container .company-name span.badge", func(e *colly.HTMLElement) {
		text := helpers.NormalizeText(e.Text)
		switch {
		case text == "Direktkunden-Projekt":
			project.DirectClient = true
		case strings.EqualFold(text, "Arbeitnehmerüberlassung") ||
			strings.EqualFold(text, "Arbeitnehmerueberlassung") ||
			strings.EqualFold(text, "ANÜ") ||
			strings.EqualFold(text, "ANUE"):
			project.ANUE = true
		}
	})

	// --- Title ----------------------------------------------------------------
	c.OnHTML("#project_container h1.margin-bottom-xs", func(e *colly.HTMLElement) {
		project.Overview.Title = strings.TrimSpace(e.Text)
	})

	// --- Company name ---------------------------------------------------------
	c.OnHTML("#project_container .company-name a", func(e *colly.HTMLElement) {
		project.Overview.Company = strings.TrimSpace(e.Text)
	})

	// --- Overview icon-list (start, end, location, rate, remote, update) ------
	c.OnHTML("#project_container .overview .icon-list li", func(e *colly.HTMLElement) {
		iconClass, _ := e.DOM.Find("i").Attr("class")
		// Strip the icon element so only the visible text remains.
		text := helpers.NormalizeText(e.DOM.Clone().Find("i").Remove().End().Text())

		switch {
		case strings.Contains(iconClass, "fa-tag"):
			project.Overview.RefNr = text
		case strings.Contains(iconClass, "fa-calendar-star"):
			project.Overview.StartDate = text
		case strings.Contains(iconClass, "fa-calendar-times"):
			project.Overview.EndDate = text
		case strings.Contains(iconClass, "fa-map-marker-alt"):
			project.Overview.Location = text
		case strings.Contains(iconClass, "fa-coins"):
			project.Rate = parseRate(text)
		case strings.Contains(iconClass, "fa-laptop-house"):
			project.Overview.Remote = text
		case strings.Contains(iconClass, "fa-history"):
			project.Overview.LastUpdate = text
		}
	})

	// --- Projektbeschreibung --------------------------------------------------
	// The first .panel-body.highlight-text inside the primary column is the
	// project description (the second one would be the skills panel).
	c.OnHTML("#project_container .col-sm-8 .panel-body.highlight-text", func(e *colly.HTMLElement) {
		if project.Description == "" {
			project.Description = strings.TrimSpace(e.Text)
			// Fallback: detect ANUE keywords in description if badge was absent.
			desc := strings.ToLower(project.Description)
			if !project.ANUE && (strings.Contains(desc, "arbeitnehmerüberlassung") ||
				strings.Contains(desc, "arbeitnehmerueberlassung") ||
				strings.Contains(desc, "anüe") ||
				strings.Contains(desc, "anue")) {
				project.ANUE = true
			}
		}
	})

	// --- Kontaktdaten / Ansprechpartner ---------------------------------------
	// #contact_data is present in the HTML even though it's hidden via
	// display:none – colly scrapes raw HTML so we can extract it directly.
	c.OnHTML("#contact_data .list-item-main", func(e *colly.HTMLElement) {
		contact := &ProjectContact{}

		// Company name sits in the .project-header h4 sibling above .list-item-main.
		company := helpers.NormalizeText(e.DOM.Closest("#contact_data").Find(".project-header h4 a").Text())
		contact.Company = company

		// Name + role sit directly inside .list-item-main, separated by a comma.
		// Remove the nested .row (email/phone/address) before reading the text.
		nameRole := helpers.NormalizeText(e.DOM.Clone().Find(".row").Remove().End().Text())
		if idx := strings.Index(nameRole, ","); idx >= 0 {
			contact.Name = strings.TrimSpace(nameRole[:idx])
			contact.Role = strings.TrimSpace(nameRole[idx+1:])
		} else {
			contact.Name = nameRole
		}

		// Email – grab from the mailto href text.
		contact.Email = strings.TrimSpace(e.DOM.Find(`a[href^="mailto:"]`).Text())

		// Phone – first .col-sm-6 holds "E-Mail: … Telefon: <number>"
		col1 := strings.TrimSpace(e.DOM.Find(".col-sm-6").First().Text())
		if idx := strings.Index(col1, "Telefon:"); idx >= 0 {
			afterPhone := strings.TrimSpace(col1[idx+len("Telefon:"):])
			// Take only the first line (the number may contain spaces, e.g. "+49 (0) 173 896 57 95").
			if nl := strings.IndexByte(afterPhone, '\n'); nl >= 0 {
				afterPhone = afterPhone[:nl]
			}
			contact.Phone = strings.TrimSpace(afterPhone)
		}

		// Address – second .col-sm-6 holds "Adresse: <street> <zip city> <country>"
		col2 := strings.TrimSpace(e.DOM.Find(".col-sm-6").Last().Text())
		if idx := strings.Index(col2, "Adresse:"); idx >= 0 {
			addr := strings.TrimSpace(col2[idx+len("Adresse:"):])
			contact.Address = strings.Join(strings.Fields(addr), " ")
		}

		project.Contact = contact
	})

	// --- Kontaktdaten old layout (col-md-6, no .list-item-main) --------------
	// Name + address sit in the first .col-md-6 as text nodes separated by <br/>.
	// Email is a mailto link; phone follows "Geschäftlich:" in the second .col-md-6.
	c.OnHTML("#contact_data", func(e *colly.HTMLElement) {
		if e.DOM.Find(".list-item-main").Length() > 0 {
			return // handled by the new-layout handler above
		}

		contact := &ProjectContact{}
		contact.Company = helpers.NormalizeText(e.DOM.Find(".project-header h4 a").Text())

		// First col-md-6: split on <br/> to separate name from address lines.
		col1HTML, _ := e.DOM.Find(".col-md-6").First().Html()
		col1HTML = strings.ReplaceAll(col1HTML, "<br/>", "\n")
		col1HTML = strings.ReplaceAll(col1HTML, "<br />", "\n")
		var lines []string
		for _, l := range strings.Split(col1HTML, "\n") {
			if t := helpers.NormalizeText(l); t != "" {
				lines = append(lines, t)
			}
		}
		linesLen := len(lines)
		if linesLen > 0 {
			contact.Name = lines[0]
			if linesLen > 1 {
				contact.Address = strings.Join(lines[1:], " ")
			}
		}

		// Second col-md-6: email via mailto link; phone after "Geschäftlich:".
		col2 := e.DOM.Find(".col-md-6").Last()
		contact.Email = strings.TrimSpace(col2.Find(`a[href^="mailto:"]`).Text())
		col2Text := col2.Text()
		if idx := strings.Index(col2Text, "Geschäftlich:"); idx >= 0 {
			after := strings.TrimSpace(col2Text[idx+len("Geschäftlich:"):])
			if nl := strings.IndexByte(after, '\n'); nl >= 0 {
				after = after[:nl]
			}
			contact.Phone = strings.TrimSpace(after)
		}

		project.Contact = contact
	})

	// --- Erforderliche Qualifikationen ----------------------------------------
	// Present only in some projects: a dedicated panel with ul.tags > li items.
	c.OnHTML("#project_container .col-sm-8 .panel", func(e *colly.HTMLElement) {
		if helpers.NormalizeText(e.DOM.Find(".panel-heading h3").Text()) != "Erforderliche Qualifikationen" {
			return
		}
		e.ForEach("ul.tags li", func(_ int, li *colly.HTMLElement) {
			if skill := helpers.NormalizeText(li.Text); skill != "" {
				project.RequiredSkills = append(project.RequiredSkills, skill)
			}
		})
	})

	// --- Kategorien und Skills ------------------------------------------------
	c.OnHTML(".project-categories a", func(e *colly.HTMLElement) {
		if skill := helpers.NormalizeText(e.Text); skill != "" {
			project.Skills = append(project.Skills, skill)
		}
	})
}
