package contractorde

import (
	"encoding/json"
	"strings"
	"time"
)

// ProjectCandidate is a single entry extracted from the listing page.
type ProjectCandidate struct {
	PlatformID   string    `json:"platform_id"   bson:"platform_id"`
	URL          string    `json:"url"            bson:"url"`
	Source       string    `json:"source"         bson:"source"`
	Title        string    `json:"title"          bson:"title"`
	Location     string    `json:"location"       bson:"location"`
	Remote       bool      `json:"is_remote"      bson:"is_remote"`
	StartDate    string    `json:"start_date"     bson:"start_date"`
	Duration     string    `json:"duration"       bson:"duration"`
	Description  string    `json:"description"    bson:"description"`
	Requirements string    `json:"requirements"   bson:"requirements"`
	ScrapedAt    time.Time `json:"scraped_at"     bson:"scraped_at"`

	Current int `json:"-"`
	Total   int `json:"-"`
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

// Project holds the fully inspected detail-page result and implements
// interfaces.Project via duck typing (no import needed).
type Project struct {
	PlatformID   string    `json:"platform_id"   bson:"platform_id"`
	URL          string    `json:"url"            bson:"url"`
	Title        string    `json:"title"          bson:"title"`
	Company      string    `json:"company"        bson:"company"`
	Description  string    `json:"description"    bson:"description"`
	Location     string    `json:"location"       bson:"location"`
	StartDate    string    `json:"start_date"     bson:"start_date"`
	Duration     string    `json:"duration"       bson:"duration"`
	Remote       bool      `json:"is_remote"      bson:"is_remote"`
	Rate         string    `json:"rate"           bson:"rate"`
	ContactName  string    `json:"contact_name"   bson:"contact_name"`
	ContactPhone string    `json:"contact_phone"  bson:"contact_phone"`
	ContactEmail string    `json:"contact_email"  bson:"contact_email"`
	ScrapedAt    time.Time `json:"scraped_at"     bson:"scraped_at"`
}

func (p *Project) GetId() string                { return "" }
func (p *Project) GetURL() string               { return p.URL }
func (p *Project) GetPlatform() string          { return Source }
func (p *Project) GetPlatformId() string        { return p.PlatformID }
func (p *Project) GetReferenceId() string       { return p.PlatformID }
// contractor.de listings don't expose a publication date — only the project's
// start_date (which can be in the future). Returning empty makes the persisted
// project's DisplayUpdatedAt fall back to ScrapedAt, so listings sort by when
// we discovered them, not by an unrelated future start.
func (p *Project) GetPlatformUpdatedAt() string { return "" }
func (p *Project) GetTitle() string             { return p.Title }
func (p *Project) GetDescription() string       { return p.Description }
func (p *Project) GetStartDate() string         { return p.StartDate }
func (p *Project) GetEndDate() string           { return "" }
func (p *Project) GetDuration() string          { return p.Duration }
func (p *Project) GetLocation() string          { return p.Location }
func (p *Project) GetSkills() []string          { return nil }
func (p *Project) GetRequiredSkills() []string  { return nil }

func (p *Project) IsDirectClient() bool { return false }
func (p *Project) IsRemote() bool {
	return p.Remote || strings.Contains(strings.ToLower(p.Location), "remote")
}
func (p *Project) IsANUE() bool { return false }

func (p *Project) GetRateRaw() string      { return p.Rate }
func (p *Project) GetRateAmount() *float64 { return nil }
func (p *Project) GetRateCurrency() string { return "" }
func (p *Project) GetRateType() string     { return "" }

func (p *Project) GetContactName() string    { return p.ContactName }
func (p *Project) GetContactCompany() string { return "Contractor Consulting GmbH" }
func (p *Project) GetContactEmail() string   { return p.ContactEmail }
func (p *Project) GetContactPhone() string   { return p.ContactPhone }
func (p *Project) GetContactRole() string    { return "" }
func (p *Project) GetContactAddress() string { return "" }
func (p *Project) GetContactImage() string   { return "" }
