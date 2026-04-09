package models

import (
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"hblabs.co/falcon/modules/interfaces"
)

// PersistedRate is the storage representation of interfaces.Rate.
type PersistedRate struct {
	Raw      string              `json:"raw"                bson:"raw"`
	Amount   *float64            `json:"amount,omitempty"   bson:"amount,omitempty"`
	Currency string              `json:"currency,omitempty" bson:"currency,omitempty"`
	Type     interfaces.RateType `json:"type,omitempty"     bson:"type,omitempty"`
}

func (r *PersistedRate) GetRaw() string               { return r.Raw }
func (r *PersistedRate) GetAmount() *float64          { return r.Amount }
func (r *PersistedRate) GetCurrency() string          { return r.Currency }
func (r *PersistedRate) GetType() interfaces.RateType { return r.Type }

// PersistedContact is the storage representation of interfaces.Contact.
type PersistedContact struct {
	Company string `json:"company,omitempty" bson:"company,omitempty"`
	Name    string `json:"name"              bson:"name"`
	Role    string `json:"role,omitempty"    bson:"role,omitempty"`
	Email   string `json:"email,omitempty"   bson:"email,omitempty"`
	Phone   string `json:"phone,omitempty"   bson:"phone,omitempty"`
	Address string `json:"address,omitempty" bson:"address,omitempty"`
}

func (c *PersistedContact) GetCompany() string { return c.Company }
func (c *PersistedContact) GetName() string    { return c.Name }
func (c *PersistedContact) GetRole() string    { return c.Role }
func (c *PersistedContact) GetEmail() string   { return c.Email }
func (c *PersistedContact) GetPhone() string   { return c.Phone }
func (c *PersistedContact) GetAddress() string { return c.Address }

// PersistedProject is the platform-agnostic struct stored in MongoDB.
// It implements interfaces.Project so it can be used anywhere a project is expected.
type PersistedProject struct {
	ID                string            `json:"id"                  bson:"id"`
	PlatformID        string            `json:"platform_id"         bson:"platform_id"`
	Platform          string            `json:"platform"            bson:"platform"`
	URL               string            `json:"url"                 bson:"url"`
	PlatformUpdatedAt string            `json:"platform_updated_at" bson:"platform_updated_at"`
	Title             string            `json:"title"               bson:"title"`
	Company           string            `json:"company"             bson:"company"`
	Description       string            `json:"description"         bson:"description"`
	StartDate         string            `json:"start_date,omitempty" bson:"start_date,omitempty"`
	EndDate           string            `json:"end_date,omitempty"   bson:"end_date,omitempty"`
	Location          string            `json:"location,omitempty"   bson:"location,omitempty"`
	Skills            []string          `json:"skills,omitempty"         bson:"skills,omitempty"`
	RequiredSkills    []string          `json:"required_skills,omitempty" bson:"required_skills,omitempty"`
	Rate              *PersistedRate    `json:"rate,omitempty"      bson:"rate,omitempty"`
	Contact           *PersistedContact `json:"contact,omitempty"   bson:"contact,omitempty"`
	DirectClient      bool              `json:"is_direct_client,omitempty" bson:"is_direct_client,omitempty"`
	Remote            bool              `json:"is_remote,omitempty" bson:"is_remote,omitempty"`
	ANUE              bool              `json:"is_anue,omitempty"   bson:"is_anue,omitempty"`
	ScrapedAt         time.Time         `json:"scraped_at"          bson:"scraped_at"`
}

// Implement interfaces.Project.
func (p *PersistedProject) GetId() string                { return p.ID }
func (p *PersistedProject) GetURL() string               { return p.URL }
func (p *PersistedProject) GetPlatform() string          { return p.Platform }
func (p *PersistedProject) GetPlatformId() string        { return p.PlatformID }
func (p *PersistedProject) GetPlatformUpdatedAt() string { return p.PlatformUpdatedAt }
func (p *PersistedProject) GetTitle() string             { return p.Title }
func (p *PersistedProject) GetCompany() string           { return p.Company }
func (p *PersistedProject) GetDescription() string       { return p.Description }
func (p *PersistedProject) GetStartDate() string         { return p.StartDate }
func (p *PersistedProject) GetEndDate() string           { return p.EndDate }
func (p *PersistedProject) GetLocation() string          { return p.Location }
func (p *PersistedProject) GetSkills() []string          { return p.Skills }
func (p *PersistedProject) GetRequiredSkills() []string  { return p.RequiredSkills }
func (p *PersistedProject) IsDirectClient() bool         { return p.DirectClient }
func (p *PersistedProject) IsRemote() bool               { return p.Remote }
func (p *PersistedProject) IsANUE() bool                 { return p.ANUE }

func (p *PersistedProject) GetRate() interfaces.Rate {
	if p.Rate == nil {
		return nil
	}
	return p.Rate
}

func (p *PersistedProject) GetContact() interfaces.Contact {
	if p.Contact == nil {
		return nil
	}
	return p.Contact
}

func (p *PersistedProject) GetEvent() *ProjectEvent {
	event := &ProjectEvent{
		ProjectID:  p.GetId(),
		Platform:   p.GetPlatform(),
		PlatformID: p.GetPlatformId(),
		Title:      p.GetTitle(),
	}

	return event
}

// NewPersistedProject builds a PersistedProject from any interfaces.Project implementation.
// platformID and platform identify the source; scrapedAt records when the fetch occurred.
// existingID preserves a previously assigned nanoId across updates; pass "" to generate a new one.
func NewPersistedProject(src interfaces.Project, platformID, platform string, scrapedAt time.Time, existingID string) *PersistedProject {
	id := existingID
	if id == "" {
		id = gonanoid.Must()
	}
	p := &PersistedProject{
		ID:                id,
		PlatformID:        platformID,
		Platform:          platform,
		URL:               src.GetURL(),
		PlatformUpdatedAt: src.GetPlatformUpdatedAt(),
		Title:             src.GetTitle(),
		Company:           src.GetCompany(),
		Description:       src.GetDescription(),
		StartDate:         src.GetStartDate(),
		EndDate:           src.GetEndDate(),
		Location:          src.GetLocation(),
		Skills:            src.GetSkills(),
		RequiredSkills:    src.GetRequiredSkills(),
		DirectClient:      src.IsDirectClient(),
		Remote:            src.IsRemote(),
		ANUE:              src.IsANUE(),
		ScrapedAt:         scrapedAt,
	}
	if r := src.GetRate(); r != nil {
		p.Rate = &PersistedRate{
			Raw:      r.GetRaw(),
			Amount:   r.GetAmount(),
			Currency: r.GetCurrency(),
			Type:     r.GetType(),
		}
	}
	if c := src.GetContact(); c != nil {
		p.Contact = &PersistedContact{
			Company: c.GetCompany(),
			Name:    c.GetName(),
			Role:    c.GetRole(),
			Email:   c.GetEmail(),
			Phone:   c.GetPhone(),
			Address: c.GetAddress(),
		}
	}
	return p
}
