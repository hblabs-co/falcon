package models

import "time"

// UserReminderKind discriminates which reminder kind a row tracks.
// One row per (user_id, kind) — the cadence loop in falcon-signal
// upserts on that compound key. Adding a future reminder type
// (e.g. "no_matches", "device_token_expired") = define the constant
// here, no schema change required.
type UserReminderKind string

const (
	// UserReminderKindCVUpload — the user signed up but never
	// uploaded a CV. Email + push, cadence:
	//   day 1 (gracia)         → no enviar
	//   day 1 ≤ T < 7 d        → 1 cada 24 h
	//   day 7 ≤ T < 30 d       → 1 cada 7 d
	//   T ≥ 30 d               → stop, marcar Stopped=true
	UserReminderKindCVUpload UserReminderKind = "cv_upload"

	// UserReminderKindLoginAfterCV — the user uploaded a CV but
	// has no registered iOS device token, meaning they either
	// never opened the app after upload or signed out / uninstalled.
	// Email-only (no push channel — that's the whole point), same
	// cadence + Berlin window as cv_upload.
	UserReminderKindLoginAfterCV UserReminderKind = "login_after_cv"
)

// UserReminder tracks how many times (and when) we've nudged a
// specific user about a specific kind of action. Lives in its own
// collection so future reminder kinds slot in without bloating the
// User doc with cv_reminder_count, match_reminder_count, etc.
//
// Unique index on (user_id, kind) — see falcon-config/indexes.go.
type UserReminder struct {
	ID      string           `json:"id"       bson:"id"`
	UserID  string           `json:"user_id"  bson:"user_id"`
	Kind    UserReminderKind `json:"kind"     bson:"kind"`
	Count   int              `json:"count"    bson:"count"`
	FirstAt time.Time        `json:"first_at" bson:"first_at"`
	LastAt  time.Time        `json:"last_at"  bson:"last_at"`
	// Stopped flips to true once the user passes the kind's terminal
	// window (e.g. 30 d for cv_upload). The reminder loop excludes
	// these from its filter so we don't re-evaluate forever.
	Stopped bool `json:"stopped" bson:"stopped"`
}
