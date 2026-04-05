package models

// ProjectEvent is published to "project.created" or "project.updated"
// by falcon-scout whenever a project is first detected or has changed.
type ProjectEvent struct {
	ProjectID  string `json:"project_id"`
	Platform   string `json:"platform"`
	PlatformID string `json:"platform_id"`
	Title      string `json:"title"`
}

// CompanyLogoDownloadRequestedEvent is published to "storage.logo.requested"
// when a company logo needs to be downloaded and stored.
type CompanyLogoDownloadRequestedEvent struct {
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
	LogoURL     string `json:"logo_url"` // absolute URL of the original logo
	Source      string `json:"source"`   // platform identifier, e.g. "freelance.de"
}

// CompanyLogoDownloadedEvent is published to "company.logo.downloaded"
// by falcon-storage after the logo has been saved to object storage.
type CompanyLogoDownloadedEvent struct {
	CompanyID      string `json:"company_id"`
	LogoStorageURL string `json:"logo_storage_url"` // public MinIO URL, empty if logo unavailable
}
