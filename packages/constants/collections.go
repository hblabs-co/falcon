package constants

const (
	MongoProjectsCollection            = "projects"
	MongoCVsCollection                 = "cvs"
	MongoScrapeFailuresCollection      = "scrape_failures"
	MongoMatchResultsCollection        = "match_results"
	MongoIOSDeviceTokensCollection     = "ios_device_tokens"
	MongoCompaniesCollection           = "companies"
	MongoNormalizedProjectsCollection  = "projects_normalized"
	MongoErrorsCollection              = "errors"
	MongoWarningsCollection            = "warnings"
	MongoUsersCollection               = "users"
	MongoUsersConfigurationsCollection = "users_configurations"
	MongoTokensCollection              = "tokens"
	MongoRealtimeStatsCollection       = "realtime_stats"
	// MongoSystemCollection stores one doc per service (keyed by
	// service_name) with self-reported metadata. Consumed by the
	// public GET /system endpoint — publish dates today, future
	// additions (version, git commit, etc.) slot in without schema
	// churn since docs are flat maps.
	MongoSystemCollection              = "system"
)

// System document field keys. Centralised so services and the /system
// endpoint agree on names — avoids the "one service writes `publishDate`,
// another writes `publish_date`" drift.
const (
	SystemFieldServiceName = "service_name"
	SystemFieldPublishDate = "publish_date"
	SystemFieldUpdatedAt   = "updated_at"
)
