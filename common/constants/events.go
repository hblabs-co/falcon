package constants

const (
	// Streams
	StreamProjects = "PROJECTS"
	StreamCVs      = "CVS"
	StreamMatches  = "MATCHES"
	StreamScrape   = "SCRAPE"

	// Subjects
	SubjectProjectCreated  = "project.created"
	SubjectProjectUpdated  = "project.updated"
	SubjectCVIndexed       = "cv.indexed"
	SubjectMatchPending    = "match.pending"
	SubjectMatchResult     = "match.result"
	SubjectScrapeFailed    = "scrape.failed"
	SubjectScrapeRequested = "scrape.requested" // full subject: scrape.requested.{platform}
)
