package models

import (
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

// Company holds the metadata and MinIO logo URL for a company/agency.
// Indexed by company_id (unique across platforms since IDs are platform-assigned).
type Company struct {
	ID           string    `json:"id"             bson:"id"`
	CompanyName  string    `json:"company_name"   bson:"company_name"`
	CompanyID    string    `json:"company_id"     bson:"company_id"`         // ID assigned by freelance.de e.g. "13525"
	Source       string    `json:"source,omitempty" bson:"source,omitempty"` // platform name, e.g. "somi.de"
	LogoMinioURL string    `json:"logo_minio_url" bson:"logo_minio_url"`     // empty if logo unavailable
	CreatedAt    time.Time `json:"created_at"     bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"     bson:"updated_at"`

	RecruiterRodeoStats *RecruiterRodeoStats `json:"recruiter_rodeo_stats,omitempty" bson:"recruiter_rodeo_stats,omitempty"`

	// Metadata fetched periodically from the company's public web presence
	// (robots.txt, security.txt, sitemap reference, etc.). Refreshed by the
	// platform scout that "owns" this company on a low-frequency loop.
	Metadata *CompanyMetadata `json:"metadata,omitempty" bson:"metadata,omitempty"`
}

// CompanyMetadata holds the snapshot of well-known files fetched from the
// company's website. Each field is optional — files that are missing or
// inaccessible (404, network failure) leave the field empty.
type CompanyMetadata struct {
	RobotsTxt   string    `json:"robots_txt,omitempty"   bson:"robots_txt,omitempty"`
	SecurityTxt string    `json:"security_txt,omitempty" bson:"security_txt,omitempty"`
	HumansTxt   string    `json:"humans_txt,omitempty"   bson:"humans_txt,omitempty"`
	SitemapURL  string    `json:"sitemap_url,omitempty"  bson:"sitemap_url,omitempty"` // extracted from robots.txt "Sitemap:" directive
	UpdatedAt   time.Time `json:"updated_at"             bson:"updated_at"`
}

func NewCompany(companyID, companyName, logoMinioURL string) *Company {
	return &Company{
		ID:           gonanoid.Must(),
		CompanyID:    companyID,
		CompanyName:  companyName,
		LogoMinioURL: logoMinioURL,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}
