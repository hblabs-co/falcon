package models

import "time"

// User is the MongoDB document for a registered or anonymous freelance user.
// Anonymous users are created at CV index time with only their email set.
type User struct {
	ID        string    `json:"id"         bson:"id"`
	Email     string    `json:"email"      bson:"email"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}
