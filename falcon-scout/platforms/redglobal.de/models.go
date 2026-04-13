package redglobalde

import (
	"encoding/json"
	"strings"
	"time"
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
	PostedAt   string    `json:"posted_at"   bson:"posted_at"` // canonical YYYY-MM-DD; normalized from listing's DD.MM.YYYY in parseListingDate
	ScrapedAt  time.Time `json:"scraped_at"  bson:"scraped_at"`

	Current int `json:"-"`
	Total   int `json:"-"`
}

func (c *ProjectCandidate) SetTotal(n int)   { c.Total = n }
func (c *ProjectCandidate) SetCurrent(n int) { c.Current = n }

// decodeCandidate converts the opaque candidate field stored in a ServiceError
// back into a typed ProjectCandidate. The field is stored as any → BSON maps
// it to bson.M on read → json round-trip converts it to our struct.
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

// // UpdateFromResult enriches the candidate with data from the detail page.
// func (c *ProjectCandidate) UpdateFromResult(result interfaces.Project) {
// 	if v := result.GetTitle(); v != "" {
// 		c.Title = v
// 	}
// 	if v := result.GetLocation(); v != "" {
// 		c.Location = v
// 	}
// }

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
	Remote      bool      `json:"is_remote"      bson:"is_remote"`
	DatePosted  string    `json:"date_posted"    bson:"date_posted"`
	EndDate     string    `json:"end_date"       bson:"end_date"`
	ReferenceID string    `json:"reference_id"   bson:"reference_id"`
	Rate        string    `json:"rate"           bson:"rate"`
	ScrapedAt   time.Time `json:"scraped_at"     bson:"scraped_at"`
}

func (p *Project) GetId() string                { return "" }
func (p *Project) GetURL() string               { return p.URL }
func (p *Project) GetPlatform() string          { return Source }
func (p *Project) GetPlatformId() string        { return p.PlatformID }
func (p *Project) GetReferenceId() string       { return p.ReferenceID }
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

func (p *Project) GetRateRaw() string      { return p.Rate }
func (p *Project) GetRateAmount() *float64 { return nil }
func (p *Project) GetRateCurrency() string { return "" }
func (p *Project) GetRateType() string     { return "" }

func (p *Project) GetContactName() string    { return "" }
func (p *Project) GetContactCompany() string { return "" }
func (p *Project) GetContactEmail() string   { return "" }
func (p *Project) GetContactPhone() string   { return "" }
func (p *Project) GetContactRole() string    { return "" }
func (p *Project) GetContactAddress() string { return "" }
