package models

import "time"

// AuthOptOutKind discriminates which family of auth-related sends
// the opt-out covers. Today only conversion_reminders exists; new
// kinds (newsletter, security_alerts, ...) slot in here without
// schema change.
type AuthOptOutKind string

const (
	// AuthOptOutKindConversionReminders silences any reminder whose
	// goal is to push the recipient through the onboarding funnel
	// (cv-upload, login-after-cv, future "verify your magic link").
	// Lives on email, not user_id, so it covers three audiences:
	//   1. user in `users` without CV
	//   2. user in `users` with CV but no login
	//   3. pure intents — pidieron magic-link, nunca verificaron, no
	//      tienen `users` doc todavía
	// Audience #3 only gets reminders the day se implemente ese loop;
	// el opt-out ya cubre el caso por adelantado.
	AuthOptOutKindConversionReminders AuthOptOutKind = "conversion_reminders"
)

// AuthOptOutSource records how the opt-out was registered. Useful
// for compliance audits ("show me every opt-out, by what mechanism")
// and for debugging unexpected silenced users.
type AuthOptOutSource string

const (
	// AuthOptOutSourceAuthenticated — user pressed an opt-out button
	// in the authenticated UI (requires JWT). User exists in `users`.
	AuthOptOutSourceAuthenticated AuthOptOutSource = "authenticated"
	// AuthOptOutSourceUnsubscribeLink — user clicked the unsubscribe
	// link embedded in an email. Usable by intents that never
	// verified, since no JWT is required.
	AuthOptOutSourceUnsubscribeLink AuthOptOutSource = "unsubscribe_link"
	// AuthOptOutSourceManual — admin set the opt-out on behalf of a
	// user (e.g. customer support request).
	AuthOptOutSourceManual AuthOptOutSource = "manual"
)

// AuthOptOut suppresses outbound mails of the given kind to the
// given email. Email is unique per (email, kind) pair — a row per
// opt-out kind, so revoking just one kind in the future doesn't
// touch the others.
//
// Lookup pattern: reminder loops bulk-fetch
//   { email: { $in: [page emails] }, kind: <reminder kind> }
// per page of candidates and build an in-memory set, instead of
// N round-trips per tick.
type AuthOptOut struct {
	ID         string           `json:"id"          bson:"id"`
	Email      string           `json:"email"       bson:"email"`
	Kind       AuthOptOutKind   `json:"kind"        bson:"kind"`
	OptedOutAt time.Time        `json:"opted_out_at" bson:"opted_out_at"`
	Source     AuthOptOutSource `json:"source"      bson:"source"`
	// UserID is set when the opt-out was made by an authenticated
	// user; empty for unsubscribe-link or manual flows on a pure
	// intent that has no `users` doc yet.
	UserID string `json:"user_id,omitempty" bson:"user_id,omitempty"`
}
