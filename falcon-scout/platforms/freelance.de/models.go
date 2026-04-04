package freelancede

import (
	"strings"
	"time"

	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/interfaces"
)

// ProjectCandidate is a single entry from the /projekte listing page.
type ProjectCandidate struct {
	PlatformID        string    `json:"platform_id"         bson:"platform_id"`
	URL               string    `json:"url"                 bson:"url"`
	Source            string    `json:"source"              bson:"source"`
	Title             string    `json:"title"               bson:"title"`
	Company           string    `json:"company"             bson:"company"`
	CompanyLogo       string    `json:"company_logo"        bson:"company_logo"`
	Skills            []string  `json:"skills"              bson:"skills"`
	StartDate         string    `json:"start_date"          bson:"start_date"`
	Location          []string  `json:"location"            bson:"location"`
	Remote            bool      `json:"remote"              bson:"remote"`
	PlatformUpdatedAt string    `json:"platform_updated_at" bson:"platform_updated_at"`
	ScrapedAt         time.Time `json:"scraped_at"          bson:"scraped_at"`
	// ExistingID carries the internal nanoId of an already-persisted project so
	// Replace can filter by id rather than platform_id, keeping the ID stable.
	ExistingID string `json:"-" bson:"-"`

	Current int
	Total   int
}

// ProjectOverview holds the quick-info items from the overview panel.
type ProjectOverview struct {
	Title      string `json:"title"                bson:"title"`
	Company    string `json:"company"              bson:"company"`
	RefNr      string `json:"ref_nr,omitempty"     bson:"ref_nr,omitempty"`
	StartDate  string `json:"start_date,omitempty" bson:"start_date,omitempty"`
	EndDate    string `json:"end_date,omitempty"   bson:"end_date,omitempty"`
	Location   string `json:"location,omitempty"   bson:"location,omitempty"`
	Remote     string `json:"remote,omitempty"     bson:"remote,omitempty"`
	LastUpdate string `json:"last_update,omitempty" bson:"last_update,omitempty"`
}

type ProjectRate struct {
	Raw      string             `json:"raw"               bson:"raw"`
	Amount   *float64           `json:"amount,omitempty"  bson:"amount,omitempty"`
	Currency string             `json:"currency,omitempty" bson:"currency,omitempty"`
	Type     constants.RateType `json:"type,omitempty"    bson:"type,omitempty"`
}

type ProjectContact struct {
	Company string `json:"company,omitempty" bson:"company,omitempty"`
	Name    string `json:"name"              bson:"name"`
	Role    string `json:"role,omitempty"    bson:"role,omitempty"`
	Email   string `json:"email,omitempty"   bson:"email,omitempty"`
	Phone   string `json:"phone,omitempty"   bson:"phone,omitempty"`
	Address string `json:"address,omitempty" bson:"address,omitempty"`
}

// Project is the top-level result persisted to MongoDB.
type Project struct {
	PlatformID     string          `json:"platform_id"              bson:"platform_id"`
	Source         string          `json:"source"                   bson:"source"`
	URL            string          `json:"url"                      bson:"url"`
	Overview       ProjectOverview `json:"overview"                 bson:"overview"`
	Description    string          `json:"description"              bson:"description"`
	Rate           *ProjectRate    `json:"rate"                     bson:"rate"`
	Skills         []string        `json:"skills,omitempty"         bson:"skills,omitempty"`
	Contact        *ProjectContact `json:"contact,omitempty"        bson:"contact,omitempty"`
	DirectClient   bool            `json:"is_direct_client,omitempty" bson:"is_direct_client,omitempty"`
	ANUE           bool            `json:"is_anue,omitempty"        bson:"is_anue,omitempty"`
	RequiredSkills []string        `json:"required_skills,omitempty" bson:"required_skills,omitempty"`
	ScrapedAt      time.Time       `json:"scraped_at"               bson:"scraped_at"`
}

func (p *Project) GetId() string                { return "" }
func (p *Project) GetURL() string               { return p.URL }
func (p *Project) GetPlatform() string          { return Source }
func (p *Project) GetPlatformId() string        { return platformIDRe.FindString(p.URL) }
func (p *Project) GetPlatformUpdatedAt() string { return p.Overview.LastUpdate }

func (p *Project) GetTitle() string            { return p.Overview.Title }
func (p *Project) GetCompany() string          { return p.Overview.Company }
func (p *Project) GetDescription() string      { return p.Description }
func (p *Project) GetStartDate() string        { return p.Overview.StartDate }
func (p *Project) GetEndDate() string          { return p.Overview.EndDate }
func (p *Project) GetLocation() string         { return p.Overview.Location }
func (p *Project) GetSkills() []string         { return p.Skills }
func (p *Project) GetRequiredSkills() []string { return p.RequiredSkills }
func (p *Project) IsDirectClient() bool        { return p.DirectClient }
func (p *Project) IsRemote() bool              { return strings.EqualFold(p.Overview.Remote, "remote") }
func (p *Project) IsANUE() bool                { return p.ANUE }

func (p *Project) GetRate() interfaces.Rate {
	if p.Rate == nil {
		return nil
	}
	return p.Rate
}

func (p *Project) GetContact() interfaces.Contact {
	if p.Contact == nil {
		return nil
	}
	return p.Contact
}

// ProjectRate implements interfaces.Rate.
func (r *ProjectRate) GetRaw() string              { return r.Raw }
func (r *ProjectRate) GetAmount() *float64         { return r.Amount }
func (r *ProjectRate) GetCurrency() string         { return r.Currency }
func (r *ProjectRate) GetType() constants.RateType { return r.Type }

// ProjectContact implements interfaces.Contact.
func (c *ProjectContact) GetCompany() string { return c.Company }
func (c *ProjectContact) GetName() string    { return c.Name }
func (c *ProjectContact) GetRole() string    { return c.Role }
func (c *ProjectContact) GetEmail() string   { return c.Email }
func (c *ProjectContact) GetPhone() string   { return c.Phone }
func (c *ProjectContact) GetAddress() string { return c.Address }
