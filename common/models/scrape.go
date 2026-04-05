package models

import "time"

// ScrapeRequestedEvent is published to NATS subject "scrape.requested.{platform}"
// by falcon-scrape-api when a user submits a URL for on-demand scraping.
// falcon-scout instances for that platform compete to process it.
type ScrapeRequestedEvent struct {
	Platform    string    `json:"platform"`
	URL         string    `json:"url"`
	RequestedAt time.Time `json:"requested_at"`
}
