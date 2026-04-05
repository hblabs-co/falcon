package models

import (
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

// Company holds the metadata and MinIO logo URL for a company/agency.
// Indexed by company_id (unique across platforms since IDs are platform-assigned).
type Company struct {
	ID           string    `json:"id"             bson:"id"`
	CompanyID    string    `json:"company_id"     bson:"company_id"` // platform-assigned ID, e.g. "13525"
	CompanyName  string    `json:"company_name"   bson:"company_name"`
	LogoMinioURL string    `json:"logo_minio_url" bson:"logo_minio_url"` // empty if logo unavailable
	CreatedAt    time.Time `json:"created_at"     bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"     bson:"updated_at"`
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
