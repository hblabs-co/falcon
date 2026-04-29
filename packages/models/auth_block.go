package models

import "time"

// AuthBlockReason describes why an email is blocked. Keep the set
// small — anything more specific belongs in a free-text note field
// in the future. Used for triage in admin UI, not for behaviour.
type AuthBlockReason string

const (
	// AuthBlockReasonAbuse — repeated patterns indicating malicious
	// use (e.g. enumerating emails, scripted floods).
	AuthBlockReasonAbuse AuthBlockReason = "abuse"
	// AuthBlockReasonSpam — flagged as a spam source by the email
	// provider, or by our own throttle thresholds firing repeatedly.
	AuthBlockReasonSpam AuthBlockReason = "spam"
	// AuthBlockReasonManual — admin decided to block, no automated
	// signal. Always paired with a non-empty ByAdmin.
	AuthBlockReasonManual AuthBlockReason = "manual"
)

// AuthBlockScope governs which auth flows the block applies to.
// Today we only enforce magic_link scope. global is reserved for
// the day a user with an existing account misbehaves and we want to
// also revoke their JWT sessions — defer wiring until that happens.
type AuthBlockScope string

const (
	// AuthBlockScopeMagicLink — reject `POST /auth/magic` for this
	// email. Does NOT touch existing JWTs (a user who already has a
	// session can keep using it).
	AuthBlockScopeMagicLink AuthBlockScope = "magic_link"
	// AuthBlockScopeGlobal — magic_link + revoke active JWTs. Not
	// implemented yet; see AUTH.md section 6.
	AuthBlockScopeGlobal AuthBlockScope = "global"
)

// AuthBlock is a single row representing "do not let this email
// authenticate". Identified by email (unique). expires_at == nil
// means permanent; otherwise the block stops applying when now is
// past expires_at — admin can clean up expired rows whenever, but
// the handler-side check ignores expired rows even if not deleted.
type AuthBlock struct {
	ID        string          `json:"id"        bson:"id"`
	Email     string          `json:"email"     bson:"email"`
	BlockedAt time.Time       `json:"blocked_at" bson:"blocked_at"`
	Reason    AuthBlockReason `json:"reason"    bson:"reason"`
	Scope     AuthBlockScope  `json:"scope"     bson:"scope"`
	// ByAdmin is the admin user_id who imposed the block. Empty
	// string when the block was inserted automatically (e.g. by a
	// future abuse-detection job).
	ByAdmin string `json:"by_admin,omitempty" bson:"by_admin,omitempty"`
	// ExpiresAt — nil = permanent. Use a pointer so the BSON layer
	// stores `null` rather than the time.Time zero value, keeping
	// "permanent" trivially queryable.
	ExpiresAt *time.Time `json:"expires_at,omitempty" bson:"expires_at,omitempty"`
}

// Active returns true if the block is currently in force at `now`.
// Permanent blocks (ExpiresAt == nil) are always active; temporary
// ones are only active while now < expires_at. Centralised here so
// the handler and admin UI agree on the semantics.
func (b *AuthBlock) Active(now time.Time) bool {
	if b.ExpiresAt == nil {
		return true
	}
	return now.Before(*b.ExpiresAt)
}
