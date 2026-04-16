package computerfuturescom

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ProjectCandidate is a single entry extracted from the API response.
type ProjectCandidate struct {
	PlatformID  string   `json:"platform_id"  bson:"platform_id"` // jobReference, e.g. "5768"
	URL         string   `json:"url"          bson:"url"`
	Source      string   `json:"source"       bson:"source"`
	Title       string   `json:"title"        bson:"title"`
	Description string   `json:"description"  bson:"description"`
	Location    string   `json:"location"     bson:"location"`
	City        string   `json:"city"         bson:"city"`
	Country     string   `json:"country"      bson:"country"`
	Remote      bool     `json:"is_remote"    bson:"is_remote"`
	StartDate   string   `json:"start_date"   bson:"start_date"`
	EndDate     string   `json:"end_date"     bson:"end_date"` // from expiryDate
	Duration    string   `json:"duration"     bson:"duration"`
	Industry    []string `json:"industry"     bson:"industry"`
	Skills      []string `json:"skills"       bson:"skills"`
	PostedAt    string   `json:"posted_at"    bson:"posted_at"` // canonical YYYY-MM-DD from postDate
	LastUpdated string   `json:"last_updated" bson:"last_updated"` // raw relative from API e.g. "Updated: 1 day ago" — TODO: use for update detection

	// Rate
	SalaryText     string  `json:"salary_text"     bson:"salary_text"`
	SalaryFrom     float64 `json:"salary_from"     bson:"salary_from"`
	SalaryTo       float64 `json:"salary_to"       bson:"salary_to"`
	SalaryCurrency string  `json:"salary_currency" bson:"salary_currency"`
	SalaryPer      string  `json:"salary_per"      bson:"salary_per"`
	SalaryBenefits string  `json:"salary_benefits" bson:"salary_benefits"`

	// Contact
	ContactName      string `json:"contact_name"       bson:"contact_name"`
	ContactEmail     string `json:"contact_email"      bson:"contact_email"`
	ApplicationEmail string `json:"application_email"  bson:"application_email"`

	ScrapedAt time.Time `json:"scraped_at" bson:"scraped_at"`
	Current   int       `json:"-"`
	Total     int       `json:"-"`
}

func (c *ProjectCandidate) SetTotal(n int)   { c.Total = n }
func (c *ProjectCandidate) SetCurrent(n int) { c.Current = n }

func decodeCandidate(raw any) (*ProjectCandidate, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var c ProjectCandidate
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Project implements interfaces.Project via duck typing.
type Project struct {
	PlatformID   string    `json:"platform_id"   bson:"platform_id"`
	URL          string    `json:"url"            bson:"url"`
	Title        string    `json:"title"          bson:"title"`
	Description  string    `json:"description"    bson:"description"`
	Location     string    `json:"location"       bson:"location"`
	Industry     string    `json:"industry"       bson:"industry"`
	StartDate    string    `json:"start_date"     bson:"start_date"`
	EndDate      string    `json:"end_date"       bson:"end_date"`
	Duration     string    `json:"duration"       bson:"duration"`
	Skills       []string  `json:"skills"         bson:"skills"`
	Remote       bool      `json:"is_remote"      bson:"is_remote"`
	PostedAt     string    `json:"posted_at"      bson:"posted_at"`
	RateRaw      string    `json:"rate_raw"       bson:"rate_raw"`
	RateAmount   *float64  `json:"rate_amount"    bson:"rate_amount"`
	RateCurrency string    `json:"rate_currency"  bson:"rate_currency"`
	RatePer      string    `json:"rate_per"       bson:"rate_per"`
	ContactName  string    `json:"contact_name"   bson:"contact_name"`
	ContactEmail string    `json:"contact_email"  bson:"contact_email"`
	ScrapedAt    time.Time `json:"scraped_at"     bson:"scraped_at"`
}

func (p *Project) GetId() string                { return "" }
func (p *Project) GetURL() string               { return p.URL }
func (p *Project) GetPlatform() string          { return Source }
func (p *Project) GetPlatformId() string        { return p.PlatformID }
func (p *Project) GetReferenceId() string       { return p.PlatformID }
func (p *Project) GetPlatformUpdatedAt() string { return p.PostedAt }
func (p *Project) GetTitle() string             { return p.Title }
func (p *Project) GetCompany() string           { return "Computer Futures, part of SThree" }
func (p *Project) GetDescription() string       { return p.Description }
func (p *Project) GetStartDate() string         { return p.StartDate }
func (p *Project) GetEndDate() string           { return p.EndDate }
func (p *Project) GetDuration() string          { return p.Duration }
func (p *Project) GetLocation() string          { return p.Location }
func (p *Project) GetSkills() []string          { return p.Skills }
func (p *Project) GetRequiredSkills() []string  { return nil }
func (p *Project) IsDirectClient() bool         { return false }
func (p *Project) IsRemote() bool {
	return p.Remote || strings.Contains(strings.ToLower(p.Location), "remote")
}
func (p *Project) IsANUE() bool { return false }

func (p *Project) GetRateRaw() string { return p.RateRaw }
func (p *Project) GetRateAmount() *float64 {
	if p.RateAmount != nil && *p.RateAmount > 0 {
		return p.RateAmount
	}
	return nil
}
func (p *Project) GetRateCurrency() string { return p.RateCurrency }
func (p *Project) GetRateType() string     { return p.RatePer }

func (p *Project) GetContactName() string    { return p.ContactName }
func (p *Project) GetContactCompany() string { return "Computer Futures, part of SThree" }
func (p *Project) GetContactEmail() string   { return p.ContactEmail }
func (p *Project) GetContactPhone() string   { return "" }
func (p *Project) GetContactRole() string    { return "" }
func (p *Project) GetContactAddress() string { return "" }
func (p *Project) GetContactImage() string   { return "" }

// candidateToProject builds a Project from a candidate — no detail page needed.
func candidateToProject(c *ProjectCandidate) *Project {
	var rateAmount *float64
	if c.SalaryFrom > 0 {
		avg := (c.SalaryFrom + c.SalaryTo) / 2
		rateAmount = &avg
	}

	currency := normalizeCurrency(c.SalaryCurrency)

	// Priority:
	// 1. Personal contactEmail (e.g. "j.mampaso@computerfutures.at") → direct to recruiter
	// 2. applicationEmail (e.g. "jimo.27973@aplitrak.com") → job-specific tracking proxy
	// 3. Generic contactEmail (e.g. "applications-de@sthree.com") → shared inbox, last resort
	email := c.ContactEmail
	if isGenericInbox(email) {
		if c.ApplicationEmail != "" {
			email = c.ApplicationEmail
		}
		// else keep the generic — better than nothing
	}
	if email == "" {
		email = c.ApplicationEmail
	}

	return &Project{
		PlatformID:   c.PlatformID,
		URL:          c.URL,
		Title:        c.Title,
		Description:  cleanDescription(c.Description),
		Location:     c.Location,
		Industry:     strings.Join(c.Industry, ", "),
		StartDate:    c.StartDate,
		EndDate:      c.EndDate,
		Duration:     c.Duration,
		Skills:       c.Skills,
		Remote:       c.Remote,
		PostedAt:     c.PostedAt,
		RateRaw:      c.SalaryText,
		RateAmount:   rateAmount,
		RateCurrency: currency,
		RatePer:      c.SalaryPer,
		ContactName:  c.ContactName,
		ContactEmail: email,
		ScrapedAt:    time.Now(),
	}
}

// cleanDescription strips the SThree boilerplate footer and tracking pixel
// from the HTML description, and converts to plain text.
func cleanDescription(html string) string {
	// Strip tracking pixel <img src="...adcourier...">
	if idx := strings.Index(html, "<img src=\"https://counter.adcourier.com"); idx > 0 {
		html = html[:idx]
	}
	// Strip SThree boilerplate
	if idx := strings.Index(html, "SThree_Germany is acting"); idx > 0 {
		html = html[:idx]
	}
	// Basic HTML → text
	html = strings.ReplaceAll(html, "<br />", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n")
	html = strings.ReplaceAll(html, "</li>", "\n")
	// Strip remaining tags
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	text := result.String()
	// Collapse whitespace
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(text)
}

// isGenericInbox returns true if the email looks like a shared inbox rather
// than a personal recruiter address. These are deprioritized in favor of the
// job-specific applicationEmail.
func isGenericInbox(email string) bool {
	lower := strings.ToLower(email)
	return strings.HasPrefix(lower, "applications") ||
		strings.HasPrefix(lower, "info@") ||
		strings.HasPrefix(lower, "jobs@") ||
		strings.HasPrefix(lower, "careers@") ||
		strings.HasPrefix(lower, "bewerbung@") ||
		strings.HasPrefix(lower, "recruiting@")
}

// normalizeCurrency maps API currency names to standard symbols.
func normalizeCurrency(s string) string {
	switch s {
	case "€":
		return "EUR"
	case "Swiss Franc":
		return "CHF"
	case "$":
		return "USD"
	case "£":
		return "GBP"
	}
	return s
}

// formatRate builds a human-readable rate string from the structured fields.
func formatRate(from, to float64, currency, per string) string {
	if from <= 0 && to <= 0 {
		return "Auf Anfrage"
	}
	cur := normalizeCurrency(currency)
	if from == to || to <= 0 {
		return fmt.Sprintf("%.0f %s/%s", from, cur, per)
	}
	return fmt.Sprintf("%.0f-%.0f %s/%s", from, to, cur, per)
}
