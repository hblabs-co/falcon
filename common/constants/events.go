package constants

const (
	// Streams
	StreamProjects = "PROJECTS"
	StreamCVs      = "CVS"
	StreamMatches  = "MATCHES"

	// Subjects
	SubjectProjectCreated = "project.created"
	SubjectProjectUpdated = "project.updated"
	SubjectCVIndexed      = "cv.indexed"
	SubjectMatchPending   = "match.pending"
	SubjectMatchResult    = "match.result"
	SubjectScrapeFailed   = "scrape.failed"
)
