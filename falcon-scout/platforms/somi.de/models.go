package somide

import (
	"encoding/json"
	"strings"
	"time"
)

// ProjectCandidate is a single entry extracted from the API response.
type ProjectCandidate struct {
	PlatformID   string `json:"platform_id"    bson:"platform_id"`
	URL          string `json:"url"            bson:"url"`
	Source       string `json:"source"         bson:"source"`
	Title        string `json:"title"          bson:"title"`
	Description  string `json:"description"    bson:"description"`
	Location     string `json:"location"       bson:"location"`
	Remote       bool   `json:"is_remote"      bson:"is_remote"`
	StartDate    string `json:"start_date"     bson:"start_date"`
	EndDate      string `json:"end_date"       bson:"end_date"`
	Duration     string `json:"duration"       bson:"duration"`
	WorkSchedule string `json:"work_schedule"  bson:"work_schedule"`
	JobLocation  string `json:"job_location"   bson:"job_location"`
	PostedAt     string `json:"posted_at"      bson:"posted_at"`

	RateOnsiteFrom        float64 `json:"rate_onsite_from"        bson:"rate_onsite_from"`
	RateOnsiteTo          float64 `json:"rate_onsite_to"          bson:"rate_onsite_to"`
	RateOnsiteType        string  `json:"rate_onsite_type"        bson:"rate_onsite_type"`
	RateOnsiteDescription string  `json:"rate_onsite_description" bson:"rate_onsite_description"`

	RateRemoteFrom        float64 `json:"rate_remote_from"        bson:"rate_remote_from"`
	RateRemoteTo          float64 `json:"rate_remote_to"          bson:"rate_remote_to"`
	RateRemoteType        string  `json:"rate_remote_type"        bson:"rate_remote_type"`
	RateRemoteDescription string  `json:"rate_remote_description" bson:"rate_remote_description"`

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
	PlatformID   string    `json:"platform_id"    bson:"platform_id"`
	URL          string    `json:"url"            bson:"url"`
	Title        string    `json:"title"          bson:"title"`
	Description  string    `json:"description"    bson:"description"`
	Location     string    `json:"location"       bson:"location"`
	StartDate    string    `json:"start_date"     bson:"start_date"`
	EndDate      string    `json:"end_date"       bson:"end_date"`
	Duration     string    `json:"duration"       bson:"duration"`
	Remote       bool      `json:"is_remote"      bson:"is_remote"`
	PostedAt     string    `json:"posted_at"      bson:"posted_at"`
	RateOnsiteFrom        float64 `json:"rate_onsite_from"        bson:"rate_onsite_from"`
	RateOnsiteTo          float64 `json:"rate_onsite_to"          bson:"rate_onsite_to"`
	RateOnsiteType        string  `json:"rate_onsite_type"        bson:"rate_onsite_type"`
	RateOnsiteDescription string  `json:"rate_onsite_description" bson:"rate_onsite_description"`

	RateRemoteFrom        float64 `json:"rate_remote_from"        bson:"rate_remote_from"`
	RateRemoteTo          float64 `json:"rate_remote_to"          bson:"rate_remote_to"`
	RateRemoteType        string  `json:"rate_remote_type"        bson:"rate_remote_type"`
	RateRemoteDescription string  `json:"rate_remote_description" bson:"rate_remote_description"`
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
func (p *Project) GetCompany() string           { return "SOMI Experts GmbH" }
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
func (p *Project) IsANUE() bool { return false }

// onsiteBlock / remoteBlock reconstruct the rateBlock view from the flat
// DTO fields. Kept as helpers so the rate-interface getters all share
// the same picking / formatting logic with buildRate.
func (p *Project) onsiteBlock() rateBlock {
	return rateBlock{
		From:        p.RateOnsiteFrom,
		To:          p.RateOnsiteTo,
		Type:        p.RateOnsiteType,
		Description: p.RateOnsiteDescription,
	}
}

func (p *Project) remoteBlock() rateBlock {
	return rateBlock{
		From:        p.RateRemoteFrom,
		To:          p.RateRemoteTo,
		Type:        p.RateRemoteType,
		Description: p.RateRemoteDescription,
	}
}

// GetRateRaw renders the combined human-readable rate string. When
// onsite and remote differ, both are shown with "(onsite)" / "(remote)"
// tags; when one side is empty or both match, a single rendering is used.
func (p *Project) GetRateRaw() string {
	return formatRateDisplay(p.onsiteBlock(), p.remoteBlock(), p.IsRemote())
}

// GetRateAmount returns the midpoint of from..to from the primary block
// (remote when the job is remote, else onsite). Nil when no rate data.
func (p *Project) GetRateAmount() *float64 {
	primary := pickPrimaryBlock(p.onsiteBlock(), p.remoteBlock(), p.IsRemote())
	switch {
	case primary.From > 0 && primary.To > 0:
		mid := (primary.From + primary.To) / 2
		return &mid
	case primary.From > 0:
		v := primary.From
		return &v
	case primary.To > 0:
		v := primary.To
		return &v
	}
	return nil
}

// GetRateCurrency returns "EUR" when any rate data exists; empty otherwise.
// somi.de is a German platform so we hardcode the currency.
func (p *Project) GetRateCurrency() string {
	if p.onsiteBlock().HasData() || p.remoteBlock().HasData() {
		return "EUR"
	}
	return ""
}

// GetRateType returns the "hourly" / "daily" of the primary block.
func (p *Project) GetRateType() string {
	return pickPrimaryBlock(p.onsiteBlock(), p.remoteBlock(), p.IsRemote()).Type
}

func (p *Project) GetContactName() string    { return p.ContactName }
func (p *Project) GetContactCompany() string { return "SOMI Experts GmbH" }
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
		RateOnsiteFrom:        c.RateOnsiteFrom,
		RateOnsiteTo:          c.RateOnsiteTo,
		RateOnsiteType:        c.RateOnsiteType,
		RateOnsiteDescription: c.RateOnsiteDescription,
		RateRemoteFrom:        c.RateRemoteFrom,
		RateRemoteTo:          c.RateRemoteTo,
		RateRemoteType:        c.RateRemoteType,
		RateRemoteDescription: c.RateRemoteDescription,
		ContactName:  c.ContactName,
		ContactRole:  c.ContactPosition,
		ContactPhone: c.ContactPhone,
		ContactEmail: c.ContactEmail,
		ContactImage: c.ContactImage,
		ScrapedAt:    time.Now(),
	}
}
