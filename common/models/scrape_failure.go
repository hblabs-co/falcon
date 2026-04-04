package models

import "time"

// ScrapeFailure is stored in MongoDB when a project detail scrape fails.
// It preserves the raw HTML so the selectors can be debugged manually.
type ScrapeFailure struct {
	ID         string    `json:"id"          bson:"id"`
	PlatformID string    `json:"platform_id" bson:"platform_id"`
	Platform   string    `json:"platform"    bson:"platform"`
	URL        string    `json:"url"         bson:"url"`
	Error      string    `json:"error"       bson:"error"`
	HTML       string    `json:"html"        bson:"html"`
	FailedAt   time.Time `json:"failed_at"   bson:"failed_at"`
}

// ScrapeFailedEvent is published to NATS subject "scrape.failed".
type ScrapeFailedEvent struct {
	FailureID  string `json:"failure_id"`
	Platform   string `json:"platform"`
	PlatformID string `json:"platform_id"`
	URL        string `json:"url"`
	Error      string `json:"error"`
}

func (f *ScrapeFailure) GetEvent() *ScrapeFailedEvent {
	return &ScrapeFailedEvent{
		FailureID:  f.ID,
		Platform:   f.Platform,
		PlatformID: f.PlatformID,
		URL:        f.URL,
		Error:      f.Error,
	}
}
