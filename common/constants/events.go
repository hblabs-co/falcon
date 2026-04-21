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
	SubjectScrapeRequested = "scrape.requested" // full subject: scrape.requested.{platform}
	SubjectScrapeScanToday = "scrape.scan_today"

	StreamStorage                       = "STORAGE"
	SubjectStorageCompanyLogoRequested  = "company_logo.requested"
	SubjectStorageCompanyLogoDownloaded = "company_logo.downloaded"
	SubjectCVPrepareRequested           = "cv.prepare.requested"
	SubjectCVIndexRequested             = "cv.index.requested"
	SubjectCVIndexed                    = "cv.indexed"
	SubjectCVNormalized                 = "cv.normalized"

	StreamSignal                     = "SIGNAL"
	SubjectSignalDeviceTokenRegister = "signal.device_token.register"
	SubjectSignalMagicLink           = "signal.magic_link"
	SubjectSignalAdminAlert          = "signal.admin_alert"
	SubjectSignalAdminTestMatch      = "signal.admin_test_match"
	SubjectSignalLiveActivityUpdate  = "signal.live_activity_update_token"
)
