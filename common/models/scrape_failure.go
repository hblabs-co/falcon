package models

// ScrapeFailedEvent is published to NATS subject "scrape.failed".
type ScrapeFailedEvent struct {
	Platform   string `json:"platform"`
	PlatformID string `json:"platform_id"`
	URL        string `json:"url"`
	Error      string `json:"error"`
}
