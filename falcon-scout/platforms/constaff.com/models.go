package constaffcom

import (
	"encoding/json"
	"strings"
	"time"
)

// ProjectCandidate is a single entry extracted from the API response.
type ProjectCandidate struct {
	PlatformID  string `json:"platform_id"  bson:"platform_id"`
	URL         string `json:"url"          bson:"url"`
	Source      string `json:"source"       bson:"source"`
	Title       string `json:"title"        bson:"title"`
	Description string `json:"description"  bson:"description"`
	Location    string `json:"location"     bson:"location"`
	Remote      bool   `json:"is_remote"    bson:"is_remote"`
	StartDate   string `json:"start_date"   bson:"start_date"`
	EndDate     string `json:"end_date"     bson:"end_date"`
	Duration    string `json:"duration"     bson:"duration"`
	PostedAt    string `json:"posted_at"    bson:"posted_at"`  // ClosingDate as canonical YYYY-MM-DD
	TypeLabel   string `json:"type_label"   bson:"type_label"` // "Freiberuflich" | "Contractor" | …
	IsANUE      bool   `json:"is_anue"      bson:"is_anue"`    // "ANÜ" flag derived from TypeStr

	ContactInitials string `json:"contact_initials" bson:"contact_initials"` // "JBA" from User
	ContactName     string `json:"contact_name"     bson:"contact_name"`
	ContactPosition string `json:"contact_position" bson:"contact_position"`
	ContactPhone    string `json:"contact_phone"    bson:"contact_phone"`
	ContactEmail    string `json:"contact_email"    bson:"contact_email"`
	ContactImage    string `json:"contact_image"    bson:"contact_image"`

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
	PlatformID  string    `json:"platform_id"  bson:"platform_id"`
	URL         string    `json:"url"          bson:"url"`
	Title       string    `json:"title"        bson:"title"`
	Description string    `json:"description"  bson:"description"`
	Location    string    `json:"location"     bson:"location"`
	StartDate   string    `json:"start_date"   bson:"start_date"`
	EndDate     string    `json:"end_date"     bson:"end_date"`
	Duration    string    `json:"duration"     bson:"duration"`
	Remote      bool      `json:"is_remote"    bson:"is_remote"`
	PostedAt    string    `json:"posted_at"    bson:"posted_at"`
	ANUE         bool      `json:"is_anue"        bson:"is_anue"`
	ContactName  string    `json:"contact_name"   bson:"contact_name"`
	ContactRole  string    `json:"contact_role"   bson:"contact_role"`
	ContactPhone string    `json:"contact_phone"  bson:"contact_phone"`
	ContactEmail string    `json:"contact_email"  bson:"contact_email"`
	ContactImage string    `json:"contact_image"  bson:"contact_image"`
	ScrapedAt    time.Time `json:"scraped_at"     bson:"scraped_at"`
}

func (p *Project) GetId() string                { return "" }
func (p *Project) GetURL() string               { return p.URL }
func (p *Project) GetPlatform() string          { return Source }
func (p *Project) GetPlatformId() string        { return p.PlatformID }
func (p *Project) GetReferenceId() string       { return p.PlatformID }
func (p *Project) GetPlatformUpdatedAt() string { return p.PostedAt }
func (p *Project) GetTitle() string             { return p.Title }
func (p *Project) GetDescription() string       { return p.Description }
func (p *Project) GetStartDate() string         { return p.StartDate }
func (p *Project) GetEndDate() string           { return p.EndDate }
func (p *Project) GetDuration() string          { return p.Duration }
func (p *Project) GetLocation() string          { return p.Location }
func (p *Project) GetSkills() []string          { return nil }
func (p *Project) GetRequiredSkills() []string  { return nil }
func (p *Project) IsDirectClient() bool         { return false }
func (p *Project) IsRemote() bool {
	return p.Remote || strings.Contains(strings.ToLower(p.Location), "remote")
}
func (p *Project) IsANUE() bool { return p.ANUE }

func (p *Project) GetRateRaw() string      { return "Auf Anfrage" }
func (p *Project) GetRateAmount() *float64 { return nil }
func (p *Project) GetRateCurrency() string { return "" }
func (p *Project) GetRateType() string     { return "" }

func (p *Project) GetContactName() string    { return p.ContactName }
func (p *Project) GetContactCompany() string { return "Constaff GmbH" }
func (p *Project) GetContactEmail() string   { return p.ContactEmail }
func (p *Project) GetContactPhone() string   { return p.ContactPhone }
func (p *Project) GetContactRole() string    { return p.ContactRole }
func (p *Project) GetContactAddress() string { return "" }
func (p *Project) GetContactImage() string   { return p.ContactImage }

func candidateToProject(c *ProjectCandidate) *Project {
	return &Project{
		PlatformID:   c.PlatformID,
		URL:          c.URL,
		Title:        c.Title,
		Description:  c.Description,
		Location:     c.Location,
		StartDate:    c.StartDate,
		EndDate:      c.EndDate,
		Duration:     c.Duration,
		Remote:       c.Remote,
		PostedAt:     c.PostedAt,
		ANUE:         c.IsANUE,
		ContactName:  c.ContactName,
		ContactRole:  c.ContactPosition,
		ContactPhone: c.ContactPhone,
		ContactEmail: c.ContactEmail,
		ContactImage: c.ContactImage,
		ScrapedAt:    time.Now(),
	}
}
