package models

import "time"

// UserConfig stores a single named configuration value for a user on a specific platform.
// Multiple documents can exist for the same user_id (one per config name per platform).
type UserConfig struct {
	UserID    string    `json:"user_id"    bson:"user_id"`
	Platform  string    `json:"platform"   bson:"platform"`
	Name      string    `json:"name"       bson:"name"`
	Value     any       `json:"value"      bson:"value"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}
