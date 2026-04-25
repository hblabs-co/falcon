package models

// ProjectEvent is published to "project.created" or "project.updated"
// by falcon-scout whenever a project is first detected or has changed.
type ProjectEvent struct {
	ProjectID  string `json:"project_id"`
	Platform   string `json:"platform"`
	PlatformID string `json:"platform_id"`
	Title      string `json:"title"`
}

// CompanyLogoDownloadRequestedEvent is published to "storage.logo.requested"
// when a company logo needs to be downloaded and stored.
type CompanyLogoDownloadRequestedEvent struct {
	CompanyID   string `json:"company_id"`
	CompanyName string `json:"company_name"`
	LogoURL     string `json:"logo_url"` // absolute URL of the original logo
	Source      string `json:"source"`   // platform identifier, e.g. "freelance.de"
}

// CompanyLogoDownloadedEvent is published to "company.logo.downloaded"
// by falcon-storage after the logo has been saved to object storage.
type CompanyLogoDownloadedEvent struct {
	CompanyID      string `json:"company_id"`
	LogoStorageURL string `json:"logo_storage_url"` // public MinIO URL, empty if logo unavailable
}

// CVPrepareRequestedEvent is published to "cv.prepare.requested" by any service
// that needs a presigned MinIO upload URL for a new CV.
type CVPrepareRequestedEvent struct {
	RequestID string `json:"request_id"` // correlation ID — echoed back in CVPreparedEvent
	Filename  string `json:"filename"`
}

// CVPreparedEvent is published to "cv.prepared" by falcon-storage
// in response to CVPrepareRequestedEvent.
type CVPreparedEvent struct {
	RequestID string `json:"request_id"`
	CVID      string `json:"cv_id"`
	UploadURL string `json:"upload_url"`
	ExpiresAt string `json:"expires_at"` // RFC3339
}

// CVIndexRequestedEvent is published to "cv.index.requested" to trigger
// async CV processing (text extraction, embedding, Qdrant upsert).
type CVIndexRequestedEvent struct {
	CVID  string `json:"cv_id"`
	Email string `json:"email"`
}

// IOSDeviceTokenRegisterEvent is published to "signal.device_token.register"
// by falcon-api when an iOS client registers or refreshes its APNs device token.
type IOSDeviceTokenRegisterEvent struct {
	UserID   string `json:"user_id"`
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
	// LiveActivityToken is optional — iOS 17.2+ push-to-start token for the
	// FalconMatchAttributes activity. Empty for older iOS.
	LiveActivityToken string `json:"live_activity_token,omitempty"`
}

// IOSDeviceTokenLogoutEvent is published to "signal.device_token.logout" when
// the user logs out from the app. Signal does NOT delete the row — the
// APNs device token itself is a property of the device, not the user.
// Instead it clears user_id + live_activity_token + live_activity_update_token
// so signal's match lookup (GetAllByField "user_id") returns nothing for
// this device until the next register re-binds it. Keyed by (device_id,
// user_id) to avoid touching another user's row on a shared device.
type IOSDeviceTokenLogoutEvent struct {
	DeviceID string `json:"device_id"`
	UserID   string `json:"user_id,omitempty"`
}

// IOSLiveActivityUpdateTokenEvent is published to "signal.live_activity_update_token"
// when iOS assigns (or clears) an update token for a running Live Activity.
// An empty token means the activity ended/dismissed — signal clears the field
// so the next push falls back to push-to-start.
type IOSLiveActivityUpdateTokenEvent struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"` // empty to clear
}

// ProjectNormalizedEvent is published to "project.normalized" by falcon-normalizer
// after a project has been enriched and written to projects_normalized.
type ProjectNormalizedEvent struct {
	ProjectID string `json:"project_id"`
	Platform  string `json:"platform"`
	Title     string `json:"title"`
}

// MatchFlippedEvent is published to "match.flipped" by falcon-match-engine
// when its periodic sweep flips stale `normalized=false` match_results to
// true. Used only to notify iOS clients (via falcon-realtime) that any
// visible "Zum Job" spinner for this project can clear — no side effects
// elsewhere.
type MatchFlippedEvent struct {
	ProjectID string `json:"project_id"`
}

// MagicLinkRequestedEvent is published to "signal.magic_link" by falcon-api
// so that falcon-signal can deliver the email.
type MagicLinkRequestedEvent struct {
	Email     string `json:"email"`
	MagicLink string `json:"magic_link"` // full deep-link URL: falcon://auth?token=<raw>
	Platform  string `json:"platform"`   // "ios", "android", "web" — extracted from User-Agent
}

// AdminAlertKind discriminates which collection an AdminAlertEvent points at.
type AdminAlertKind string

const (
	// AdminAlertKindError points at a document in the errors collection.
	AdminAlertKindError AdminAlertKind = "error"
	// AdminAlertKindWarning points at a document in the warnings collection.
	AdminAlertKindWarning AdminAlertKind = "warning"
)

// AdminAlertEvent is published to "signal.admin_alert" by any service that
// needs to escalate a high-severity issue to the operations team.
//
// The event is a tiny discriminated union: Kind tells signal which collection
// to look in (errors or warnings) and ID is the document id. falcon-signal
// loads the full record, builds the email/push payload (translation,
// formatting, severity routing) and fans it out to every email in ADMIN_EMAILS
// via mail and — when the admin has the iOS app installed — push notification.
// Keeping the event tiny avoids duplicating ServiceError/ServiceWarning fields
// here and centralizes presentation logic in signal where templates live.
//
// Publishers should only emit this for high or critical events — there is no
// dedup yet, so flooding will reach the admins.
type AdminAlertEvent struct {
	Kind AdminAlertKind `json:"kind"`
	ID   string         `json:"id"`
}

// AdminTestMatchEvent is published to "signal.admin_test_match" by the admin
// POST /signal/test-last-match endpoint. Index picks which of the admin's own
// match_results to push, ordered by scored_at desc (same order the iOS list
// shows): 0 = latest (default), 1 = second latest, 2 = third, etc.
type AdminTestMatchEvent struct {
	Index int `json:"index"`
}
