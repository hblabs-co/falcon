package models

import "time"

// User is the MongoDB document for a registered or anonymous freelance user.
// Anonymous users are created at CV index time with only their email set.
type User struct {
	ID        string    `json:"id"         bson:"id"`
	Email     string    `json:"email"      bson:"email"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`

	// CVUploaded mirrors whether a `cvs` doc with status=indexed
	// exists for this user_id. Cheap denormalisation so the cv-
	// reminder loop can filter `{cv_uploaded: false}` without a
	// per-user join into `cvs`. Flipped to true by falcon-storage
	// when an index succeeds; reconciled at boot by falcon-config's
	// ensureUserCVFlag for any pre-existing user that already has
	// a CV but predates this field.
	CVUploaded bool `json:"cv_uploaded" bson:"cv_uploaded"`

	// LastLoggedInAt records the last time the user successfully
	// completed magic-link → JWT exchange. Durable on the User doc
	// (NOT on tokens, which are TTL-pruned when JWTs expire) so the
	// login-reminder loop can tell apart "never logged in" (target
	// for the reminder) from "logged in months ago, JWT expired"
	// (already onboarded — leave alone).
	//
	// Set by falcon-api/auth at every successful /auth/verify.
	// Backfilled at boot by falcon-config's ensureUserLastLogin for
	// any user with a currently-live JWT but missing this field.
	LastLoggedInAt time.Time `json:"last_logged_in_at,omitempty" bson:"last_logged_in_at,omitempty"`
}
