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

// CVPrepareRequestedEvent is published to "cv.prepare.requested" by any service
// that needs a presigned MinIO upload URL for a new CV.
type CVPrepareRequestedEvent struct {
	RequestID string `json:"request_id"` // correlation ID — echoed back in CVPreparedEvent
	Filename  string `json:"filename"`
}

// CVPreparedEvent is published to "cv.prepared" by falcon-storage
// in response to CVPrepareRequestedEvent.
type CVPreparedEvent struct {
	RequestID string `json:"request_id"`
	CVID      string `json:"cv_id"`
	UploadURL string `json:"upload_url"`
	ExpiresAt string `json:"expires_at"` // RFC3339
}

// CVIndexRequestedEvent is published to "cv.index.requested" to trigger
// async CV processing (text extraction, embedding, Qdrant upsert).
type CVIndexRequestedEvent struct {
	CVID  string `json:"cv_id"`
	Email string `json:"email"`
}

// DeviceTokenRegisterEvent is published to "signal.device_token.register"
// by falcon-api when a client registers or refreshes its APNs device token.
type DeviceTokenRegisterEvent struct {
	UserID string `json:"user_id"`
	Token  string `json:"token"`
}

// ProjectNormalizedEvent is published to "project.normalized" by falcon-normalizer
// after a project has been enriched and written to projects_normalized.
type ProjectNormalizedEvent struct {
	ProjectID string `json:"project_id"`
	Platform  string `json:"platform"`
	Title     string `json:"title"`
}

// MagicLinkRequestedEvent is published to "signal.magic_link" by falcon-api
// so that falcon-signal can deliver the email.
type MagicLinkRequestedEvent struct {
	Email     string `json:"email"`
	MagicLink string `json:"magic_link"` // full deep-link URL: falcon://auth?token=<raw>
	Platform  string `json:"platform"`   // "ios", "android", "web" — extracted from User-Agent
}
