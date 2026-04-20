package models

import (
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"hblabs.co/falcon/common/helpers"
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
	Image   string `json:"image,omitempty"   bson:"image,omitempty"`
}

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
	ReferenceID       string            `json:"reference_id"        bson:"reference_id"`
	Platform          string            `json:"platform"            bson:"platform"`
	URL               string            `json:"url"                 bson:"url"`
	PlatformUpdatedAt time.Time         `json:"platform_updated_at" bson:"platform_updated_at"`
	DisplayUpdatedAt  time.Time         `json:"display_updated_at"  bson:"display_updated_at"`
	Title             string            `json:"title"               bson:"title"`
	Description       string            `json:"description"         bson:"description"`
	StartDate         string            `json:"start_date,omitempty"  bson:"start_date,omitempty"`
	EndDate           string            `json:"end_date,omitempty"    bson:"end_date,omitempty"`
	Duration          string            `json:"duration,omitempty"    bson:"duration,omitempty"`
	Location          string            `json:"location,omitempty"    bson:"location,omitempty"`
	Skills            []string          `json:"skills,omitempty"         bson:"skills,omitempty"`
	RequiredSkills    []string          `json:"required_skills,omitempty" bson:"required_skills,omitempty"`
	Rate              *PersistedRate    `json:"rate,omitempty"      bson:"rate,omitempty"`
	Contact           *PersistedContact `json:"contact,omitempty"   bson:"contact,omitempty"`
	DirectClient      bool              `json:"is_direct_client" bson:"is_direct_client"`
	Remote            bool              `json:"is_remote"        bson:"is_remote"`
	ANUE              bool              `json:"is_anue"          bson:"is_anue"`
	ScrapedAt         time.Time         `json:"scraped_at"          bson:"scraped_at"`
}

// Implement interfaces.Project.
func (p *PersistedProject) GetId() string          { return p.ID }
func (p *PersistedProject) GetURL() string         { return p.URL }
func (p *PersistedProject) GetPlatform() string    { return p.Platform }
func (p *PersistedProject) GetPlatformId() string  { return p.PlatformID }
func (p *PersistedProject) GetReferenceId() string { return p.ReferenceID }
func (p *PersistedProject) GetPlatformUpdatedAt() string {
	return p.PlatformUpdatedAt.Format(time.RFC3339)
}
func (p *PersistedProject) GetTitle() string            { return p.Title }
func (p *PersistedProject) GetDescription() string      { return p.Description }
func (p *PersistedProject) GetStartDate() string        { return p.StartDate }
func (p *PersistedProject) GetEndDate() string          { return p.EndDate }
func (p *PersistedProject) GetDuration() string         { return p.Duration }
func (p *PersistedProject) GetLocation() string         { return p.Location }
func (p *PersistedProject) GetSkills() []string         { return p.Skills }
func (p *PersistedProject) GetRequiredSkills() []string { return p.RequiredSkills }
func (p *PersistedProject) IsDirectClient() bool        { return p.DirectClient }
func (p *PersistedProject) IsRemote() bool              { return p.Remote }
func (p *PersistedProject) IsANUE() bool                { return p.ANUE }

func (p *PersistedProject) GetRateRaw() string {
	if p.Rate == nil {
		return ""
	}
	return p.Rate.Raw
}
func (p *PersistedProject) GetRateAmount() *float64 {
	if p.Rate == nil {
		return nil
	}
	return p.Rate.Amount
}
func (p *PersistedProject) GetRateCurrency() string {
	if p.Rate == nil {
		return ""
	}
	return p.Rate.Currency
}
func (p *PersistedProject) GetRateType() string {
	if p.Rate == nil {
		return ""
	}
	return string(p.Rate.Type)
}

func (p *PersistedProject) GetContactName() string {
	if p.Contact == nil {
		return ""
	}
	return p.Contact.Name
}
func (p *PersistedProject) GetContactCompany() string {
	if p.Contact == nil {
		return ""
	}
	return p.Contact.Company
}
func (p *PersistedProject) GetContactEmail() string {
	if p.Contact == nil {
		return ""
	}
	return p.Contact.Email
}
func (p *PersistedProject) GetContactPhone() string {
	if p.Contact == nil {
		return ""
	}
	return p.Contact.Phone
}
func (p *PersistedProject) GetContactRole() string {
	if p.Contact == nil {
		return ""
	}
	return p.Contact.Role
}
func (p *PersistedProject) GetContactAddress() string {
	if p.Contact == nil {
		return ""
	}
	return p.Contact.Address
}
func (p *PersistedProject) GetContactImage() string {
	if p.Contact == nil {
		return ""
	}
	return p.Contact.Image
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
// If existing is non-nil, its ID is preserved and DisplayUpdatedAt is computed against
// the previous record (so re-scrapes don't shift a job's position within its day).
func NewPersistedProject(src interfaces.Project, existing *PersistedProject) *PersistedProject {
	p := &PersistedProject{
		ID:                resolveID(existing),
		PlatformID:        src.GetPlatformId(),
		ReferenceID:       src.GetReferenceId(),
		Platform:          src.GetPlatform(),
		URL:               src.GetURL(),
		PlatformUpdatedAt: helpers.ParsePlatformTime(src.GetPlatformUpdatedAt()),
		ScrapedAt:         time.Now().UTC(),
	}
	p.applySource(src)
	p.computeDisplayUpdatedAt(existing)
	return p
}

// resolveID returns the existing nanoid if available, otherwise generates a new one.
func resolveID(existing *PersistedProject) string {
	if existing != nil && existing.ID != "" {
		return existing.ID
	}
	return gonanoid.Must()
}

// applySource fills the project's content fields from the platform's interfaces.Project.
// Identity fields (ID, PlatformID, Platform, ReferenceID, URL, PlatformUpdatedAt, ScrapedAt)
// are set by NewPersistedProject before this is called.
func (p *PersistedProject) applySource(src interfaces.Project) {
	p.Title = src.GetTitle()
	p.Description = src.GetDescription()
	p.StartDate = src.GetStartDate()
	p.EndDate = src.GetEndDate()
	p.Duration = src.GetDuration()
	p.Location = src.GetLocation()
	p.Skills = src.GetSkills()
	p.RequiredSkills = src.GetRequiredSkills()
	p.DirectClient = src.IsDirectClient()
	p.Remote = src.IsRemote()
	p.ANUE = src.IsANUE()

	if raw := src.GetRateRaw(); raw != "" {
		p.Rate = &PersistedRate{
			Raw:      raw,
			Amount:   src.GetRateAmount(),
			Currency: src.GetRateCurrency(),
			Type:     interfaces.RateType(src.GetRateType()),
		}
	}
	if name := src.GetContactName(); name != "" {
		p.Contact = &PersistedContact{
			Name:    name,
			Company: src.GetContactCompany(),
			Role:    src.GetContactRole(),
			Email:   src.GetContactEmail(),
			Phone:   src.GetContactPhone(),
			Address: src.GetContactAddress(),
			Image:   src.GetContactImage(),
		}
	}
}

// computeDisplayUpdatedAt fills DisplayUpdatedAt based on PlatformUpdatedAt:
//   - If the platform timestamp has a time component (hours/min/sec), use it as-is.
//   - If it's date-only (00:00:00 after parsing), combine the date with the current
//     time-of-day so the job has a stable, plausible position within its day.
//   - If existing is non-nil and the platform date hasn't changed, preserve the
//     existing DisplayUpdatedAt time-of-day so re-scrapes don't shift the order.
//
// PlatformUpdatedAt remains the honest source of truth from the publisher.
// DisplayUpdatedAt is what the UI should sort and render on.
func (p *PersistedProject) computeDisplayUpdatedAt(existing *PersistedProject) {
	if p.PlatformUpdatedAt.IsZero() {
		// Could not parse — fall back to scraped time so the job is still sortable.
		p.DisplayUpdatedAt = p.ScrapedAt
		return
	}

	// Re-scrape with same date → preserve previous display time-of-day.
	if existing != nil && helpers.SameDate(existing.PlatformUpdatedAt, p.PlatformUpdatedAt) {
		p.DisplayUpdatedAt = existing.DisplayUpdatedAt
		return
	}

	if helpers.HasTimeComponent(p.PlatformUpdatedAt) {
		p.DisplayUpdatedAt = p.PlatformUpdatedAt
		return
	}

	// Date-only: combine the platform date with the current time-of-day.
	now := time.Now().UTC()
	p.DisplayUpdatedAt = time.Date(
		p.PlatformUpdatedAt.Year(), p.PlatformUpdatedAt.Month(), p.PlatformUpdatedAt.Day(),
		now.Hour(), now.Minute(), now.Second(), 0,
		time.UTC,
	)
}
