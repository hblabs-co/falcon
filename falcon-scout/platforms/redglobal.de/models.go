package redglobalde

import (
	"strings"
	"time"

	"hblabs.co/falcon/modules/interfaces"
)

// ProjectCandidate is a single entry extracted from the listing page.
type ProjectCandidate struct {
	PlatformID string    `json:"platform_id" bson:"platform_id"` // short ID from URL, e.g. "CX7F0rXz"
	Slug       string    `json:"slug"        bson:"slug"`        // full slug, e.g. "m365-rpa-programmierer-teilzeit-remote-asap"
	URL        string    `json:"url"         bson:"url"`
	Source     string    `json:"source"      bson:"source"`
	Title      string    `json:"title"       bson:"title"`
	Location   string    `json:"location"    bson:"location"`
	Rate       string    `json:"rate"        bson:"rate"`
	PostedAt   string    `json:"posted_at"   bson:"posted_at"` // date string from listing, e.g. "19.03.2026"
	ScrapedAt  time.Time `json:"scraped_at"  bson:"scraped_at"`

	Current int `json:"-"`
	Total   int `json:"-"`
}

// UpdateFromResult enriches the candidate with data from the detail page.
func (c *ProjectCandidate) UpdateFromResult(result interfaces.Project) {
	if v := result.GetTitle(); v != "" {
		c.Title = v
	}
	if v := result.GetLocation(); v != "" {
		c.Location = v
	}
}

// jobPostingLD maps the JSON-LD JobPosting schema found in detail pages.
type jobPostingLD struct {
	Context            string       `json:"@context"`
	Type               string       `json:"@type"`
	DatePosted         string       `json:"datePosted"`
	Description        string       `json:"description"`
	EmploymentType     string       `json:"employmentType"`
	URL                string       `json:"url"`
	Title              string       `json:"title"`
	HiringOrganization string       `json:"hiringOrganization"`
	JobLocation        *jobLocation `json:"jobLocation"`
	ValidThrough       string       `json:"validThrough"`
	Industry           string       `json:"industry"`
	JobLocationType    string       `json:"jobLocationType"`
}

type jobLocation struct {
	Type    string      `json:"@type"`
	Address *jobAddress `json:"address"`
}

type jobAddress struct {
	Type            string `json:"@type"`
	AddressLocality string `json:"addressLocality"`
	AddressRegion   string `json:"addressRegion"`
}

// Project holds the fully inspected detail-page result and implements interfaces.Project.
type Project struct {
	PlatformID  string    `json:"platform_id"  bson:"platform_id"`
	Slug        string    `json:"slug"         bson:"slug"`
	URL         string    `json:"url"          bson:"url"`
	Title       string    `json:"title"        bson:"title"`
	Company     string    `json:"company"      bson:"company"`
	Description string    `json:"description"  bson:"description"`
	Location    string    `json:"location"     bson:"location"`
	Industry    string    `json:"industry"     bson:"industry"`
	Remote      bool      `json:"is_remote"    bson:"is_remote"`
	DatePosted  string    `json:"date_posted"  bson:"date_posted"`
	EndDate     string    `json:"end_date"     bson:"end_date"`
	ScrapedAt   time.Time `json:"scraped_at"   bson:"scraped_at"`
}

func (p *Project) GetId() string                { return "" }
func (p *Project) GetURL() string               { return p.URL }
func (p *Project) GetPlatform() string          { return Source }
func (p *Project) GetPlatformId() string        { return p.PlatformID }
func (p *Project) GetPlatformUpdatedAt() string { return p.DatePosted }

func (p *Project) GetTitle() string       { return p.Title }
func (p *Project) GetCompany() string     { return p.Company }
func (p *Project) GetDescription() string { return p.Description }
func (p *Project) GetStartDate() string   { return p.DatePosted }
func (p *Project) GetEndDate() string     { return p.EndDate }

func (p *Project) GetLocation() string {
	return p.Location
}

func (p *Project) GetSkills() []string         { return nil }
func (p *Project) GetRequiredSkills() []string { return nil }

func (p *Project) IsDirectClient() bool { return false }
func (p *Project) IsRemote() bool {
	return p.Remote || strings.EqualFold(p.Location, "TELECOMMUTE")
}
func (p *Project) IsANUE() bool { return false }

func (p *Project) GetRate() interfaces.Rate       { return nil }
func (p *Project) GetContact() interfaces.Contact { return nil }

// Compile-time check.
var _ interfaces.Project = (*Project)(nil)
