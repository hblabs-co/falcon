package constants

const (
	StreamProjects           = "PROJECTS"
	SubjectProjectCreated    = "project.created"
	SubjectProjectUpdated    = "project.updated"
	SubjectProjectNormalized = "project.normalized"

	StreamMatches       = "MATCHES"
	SubjectMatchPending = "match.pending"
	SubjectMatchResult  = "match.result"

	StreamScrape           = "SCRAPE"
	SubjectScrapeFailed    = "scrape.failed"
	SubjectScrapeRequested = "scrape.requested" // full subject: scrape.requested.{platform}

	StreamStorage                       = "STORAGE"
	SubjectStorageCompanyLogoRequested  = "company_logo.requested"
	SubjectStorageCompanyLogoDownloaded = "company_logo.downloaded"
	SubjectCVPrepareRequested           = "cv.prepare.requested"
	SubjectCVIndexRequested             = "cv.index.requested"
	SubjectCVIndexed                    = "cv.indexed"

	StreamSignal                     = "SIGNAL"
	SubjectSignalDeviceTokenRegister = "signal.device_token.register"
)
