package constants

const (
	StreamProjects           = "PROJECTS"
	SubjectProjectCreated    = "project.created"
	SubjectProjectUpdated    = "project.updated"
	SubjectProjectNormalized = "project.normalized"

	StreamMatches       = "MATCHES"
	SubjectMatchPending = "match.pending"
	SubjectMatchResult  = "match.result"
	// SubjectMatchFlipped is emitted by falcon-match-engine when its
	// periodic sweep catches a match whose `normalized` flag was stale
	// and flips it to true. Only falcon-realtime subscribes — it
	// forwards to iOS clients so any visible "waiting for normalizer"
	// spinner clears without needing a refresh. Separate subject so
	// signal/etc. don't re-process a project.normalized event we're
	// only using to reach the app.
	SubjectMatchFlipped = "match.flipped"

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
	SubjectSignalDeviceTokenLogout   = "signal.device_token.logout"
	SubjectSignalMagicLink           = "signal.magic_link"
	SubjectSignalAdminAlert          = "signal.admin_alert"
	SubjectSignalAdminTestMatch      = "signal.admin_test_match"
	SubjectSignalLiveActivityUpdate  = "signal.live_activity_update_token"

	// StreamRealtime carries ephemeral client-activity events captured by
	// falcon-realtime (session_started, call, email, view_detail, etc.) so
	// they can be persisted AND optionally consumed by other services later.
	// Short MaxAge — these are stats, not transactional work.
	StreamRealtime         = "REALTIME"
	SubjectRealtimeEvent   = "realtime.event"
)
