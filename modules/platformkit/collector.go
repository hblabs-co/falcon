package platformkit

import "time"

// CollectorConfig holds the shared defaults for colly collectors across all
// HTML-scraping platforms. Each platform passes its own AllowedDomains and
// DomainGlob; the timing values come from here so they're consistent and
// only need to be tuned in one place.
type CollectorConfig struct {
	AllowedDomains []string
	DomainGlob     string
	Delay          time.Duration
	RandomDelay    time.Duration
	Parallelism    int
}

// DefaultCollectorConfig returns the standard timing values for a polite
// scraper. Platforms only need to fill AllowedDomains and DomainGlob.
func DefaultCollectorConfig(allowedDomains []string, domainGlob string) CollectorConfig {
	return CollectorConfig{
		AllowedDomains: allowedDomains,
		DomainGlob:     domainGlob,
		Delay:          1 * time.Second,
		RandomDelay:    1 * time.Second,
		Parallelism:    1,
	}
}
