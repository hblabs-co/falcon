// Package users hosts the user-centric admin surface served by
// falcon-admin: search, devices, and CV download. Auth-domain CRUD
// (magic links + JWT sessions) lives in `packages/auth` and is
// mounted directly by service.go; this package only owns the
// per-user views that compose those primitives. Mounted under the
// admin's bearer-protected route group via Mount().
//
// Split across files for readability:
//
//	types.go    — JSON view structs returned to the UI
//	routes.go   — Mount() — the route table
//	search.go   — /users/search (autocomplete)
//	detail.go   — /users/:id (header info + counts)
//	devices.go  — APNs device listing
//	cv.go       — presigned CV download
//	errors.go   — shared respondInternal helper
package users

import "time"

// userView is the autocomplete row. Hash/raw token never appear here
// — only the metadata the operator needs to pick the right user.
type userView struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name,omitempty"`
	LastName  string    `json:"last_name,omitempty"`
	JoinedAt  time.Time `json:"joined_at"`
}

// userDetail is what /users/:id returns: the row itself plus the
// counts the UI needs for header badges. HasCV gates the "download
// CV" button — the file may exist in Mongo but not be downloadable
// (no minio key persisted). CVReminder, when present, summarises
// what the cv-reminder loop has done for this user (count, last
// send, terminal flag) so the nest UI can render "Reminded N× —
// last 2d ago" next to the "No CV" badge.
type userDetail struct {
	userView
	ActiveTokens   int                 `json:"active_tokens"`
	ActiveSessions int                 `json:"active_sessions"`
	DeviceCount    int                 `json:"device_count"`
	HasCV          bool                `json:"has_cv"`
	CVFilename     string              `json:"cv_filename,omitempty"`
	CVReminder     *cvReminderSummary  `json:"cv_reminder,omitempty"`
}

// cvReminderSummary mirrors models.UserReminder fields the UI cares
// about. Kept here (not in models) because it's an admin-API view
// shape; if other callers need it later we can promote it.
type cvReminderSummary struct {
	Count   int       `json:"count"`
	FirstAt time.Time `json:"first_at,omitempty"`
	LastAt  time.Time `json:"last_at,omitempty"`
	Stopped bool      `json:"stopped"`
}

// deviceView is the per-row representation for the devices panel.
// The raw push token is masked — the operator only needs to
// recognise the device, not impersonate it.
//
// Platform tells the UI which icon to render (apple for ios,
// smartphone for android, monitor for web). Today we only persist
// iOS device tokens, so this is hardcoded "ios" — when the model
// grows to include FCM (android) and Web Push, this field becomes
// the discriminator and `token_masked` switches from APNs to the
// platform's native push handle.
type deviceView struct {
	ID                string    `json:"id"`
	DeviceID          string    `json:"device_id"`
	Platform          string    `json:"platform"`
	TokenMasked       string    `json:"token_masked,omitempty"`
	HasLiveActivity   bool      `json:"has_live_activity,omitempty"`
	HasUpdateActivity bool      `json:"has_update_activity,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}
