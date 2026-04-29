package models

import (
	"time"

	"hblabs.co/falcon/packages/clientmeta"
)

// AuthIntent is one row per `POST /auth/magic` attempt, written
// regardless of whether the user is blocked, throttled, verifies
// the link, or never opens it. Append-only — no updates, no
// automatic deletes. The collection has no TTL by design: we want
// to keep the email visible past the 15-min magic_link expiry so
// abuse detection, customer support, conversion analytics, and
// transactional follow-ups can keep working. See AUTH.md
// for the full rationale.
//
// `email` is intentionally NOT unique — duplicates are the signal
// (rate, abuse pattern, mail-not-arriving, user confused).
//
// Context is a snapshot of state at request time. We don't
// reference users by id because the user may not exist yet (or
// may be created later), and snapshotting bools beats a join
// per-row when reviewing in admin UI.
//
// Client is the bag of HTTP request metadata captured via
// clientmeta.Capture(c) — IP, user-agent, Cloudflare headers,
// Client Hints, etc. See packages/clientmeta for the full list
// and the reason each field exists.
type AuthIntent struct {
	ID          string    `json:"id"           bson:"id"`
	Email       string    `json:"email"        bson:"email"`
	RequestedAt time.Time `json:"requested_at" bson:"requested_at"`
	// DeviceID is mandatory for iOS (the only client today). When
	// web/Android land, the handler decides a fallback (probably a
	// persistent client-side fingerprint) — this field stays
	// present but the source changes.
	DeviceID string                `json:"device_id" bson:"device_id"`
	Platform string                `json:"platform"  bson:"platform"`
	Client   clientmeta.ClientMeta `json:"client"    bson:"client"`
	Context  AuthIntentContext     `json:"context"   bson:"context"`
}

// AuthIntentContext is the snapshot of relevant state at the moment
// the user pressed "request magic link". Joining users at admin-
// query time gives current state; this gives history.
type AuthIntentContext struct {
	UserExistedAtRequest bool `json:"user_existed_at_request" bson:"user_existed_at_request"`
	CVUploadedAtRequest  bool `json:"cv_uploaded_at_request"  bson:"cv_uploaded_at_request"`
}
