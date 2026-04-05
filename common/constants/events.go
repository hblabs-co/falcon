package constants

const (
	// Streams
	StreamProjects = "PROJECTS"
	StreamCVs      = "CVS"
	StreamMatches  = "MATCHES"
	StreamScrape   = "SCRAPE"
	StreamStorage  = "STORAGE"

	// Subjects
	SubjectProjectCreated               = "project.created"
	SubjectProjectUpdated               = "project.updated"
	SubjectCVIndexed                    = "cv.indexed"
	SubjectMatchPending                 = "match.pending"
	SubjectMatchResult                  = "match.result"
	SubjectScrapeFailed                 = "scrape.failed"
	SubjectScrapeRequested              = "scrape.requested" // full subject: scrape.requested.{platform}
	SubjectStorageCompanyLogoRequested  = "storage.company_logo.requested"
	SubjectStorageCompanyLogoDownloaded = "storage.company_logo.downloaded"
)
