// Package users hosts the user-centric admin surface served by
// falcon-admin: search, magic links, JWT sessions, devices, and
// CV download. Mounted under the admin's bearer-protected
// route group via Mount().
//
// Split across files for readability:
//
//	types.go    — JSON view structs returned to the UI
//	routes.go   — Mount() — the route table
//	search.go   — /users/search (autocomplete)
//	detail.go   — /users/:id (header info + counts)
//	tokens.go   — magic-link CRUD + MintTestLink helper
//	sessions.go — JWT session listing + revocation
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
// (no minio key persisted).
type userDetail struct {
	userView
	ActiveTokens   int    `json:"active_tokens"`
	ActiveSessions int    `json:"active_sessions"`
	DeviceCount    int    `json:"device_count"`
	HasCV          bool   `json:"has_cv"`
	CVFilename     string `json:"cv_filename,omitempty"`
}

// tokenView is the per-row representation used by both the magic-
// link panel and the JWT-session panel — same shape, different
// scope on the server side. Every metadata field on the underlying
// `tokens` document is surfaced (except `token_hash`, which is
// sensitive). Empty fields use omitempty so the UI can hide them
// without a presence check — historical rows often miss user_id,
// device_id, or test, since the schema grew over time.
//
// Conceptually:
//
//   - magic_link tokens are short-lived single-use credentials that
//     are exchanged for a JWT on /auth/verify. `used` flips to true
//     after the first redemption (test=true skips that flip so the
//     same link survives reinstalls).
//   - jwt tokens are the persisted user sessions returned to the
//     iOS app. They live until expires_at unless revoked.
type tokenView struct {
	ID        string    `json:"id"`
	Type      string    `json:"type,omitempty"`
	Email     string    `json:"email,omitempty"`
	DeviceID  string    `json:"device_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Platform  string    `json:"platform,omitempty"`
	Used      bool      `json:"used"`
	Revoked   bool      `json:"revoked"`
	Expired   bool      `json:"expired"`
	Test      bool      `json:"test,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// Link is the deep-link URL (`<scheme>://auth?token=<raw>`) for
	// test magic-link tokens. Absent on JWT sessions and on
	// production magic links (those only persist the hash). The UI
	// uses Link's presence to decide whether the row is
	// click-to-copy.
	Link string `json:"link,omitempty"`
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
