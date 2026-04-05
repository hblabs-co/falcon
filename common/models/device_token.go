package models

import "time"

// DeviceToken stores an iOS device token for a given user.
// Registered by the iOS app on launch via POST /device-token in falcon-signal.
type DeviceToken struct {
	ID        string    `json:"id"         bson:"id"`
	UserID    string    `json:"user_id"    bson:"user_id"`
	Token     string    `json:"token"      bson:"token"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}
