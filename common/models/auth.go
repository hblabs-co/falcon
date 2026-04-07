package models

import "time"

// TokenType identifies the purpose of a persisted token document.
type TokenType string

const (
	TokenTypeMagicLink TokenType = "magic_link"
	TokenTypeJWT       TokenType = "jwt"
)

// Token is the MongoDB document for any auth token (magic link or JWT session).
type Token struct {
	ID        string    `json:"id"         bson:"id"`
	Type      TokenType `json:"type"       bson:"type"`
	UserID    string    `json:"user_id,omitempty"    bson:"user_id,omitempty"`
	Email     string    `json:"email"      bson:"email"`
	TokenHash string    `json:"token_hash" bson:"token_hash"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
	Used      bool      `json:"used"       bson:"used"`
	Revoked   bool      `json:"revoked"    bson:"revoked"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
}
