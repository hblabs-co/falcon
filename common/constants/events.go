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

	StreamStorage                       = "STORAGE"
	SubjectStorageCompanyLogoRequested  = "company_logo.requested"
	SubjectStorageCompanyLogoDownloaded = "company_logo.downloaded"
	SubjectCVPrepareRequested           = "cv.prepare.requested"
	SubjectCVPrepared                   = "cv.prepared"
	SubjectCVIndexRequested             = "cv.index.requested"
)
