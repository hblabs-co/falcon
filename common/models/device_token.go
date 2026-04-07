package models

import "time"

// IOSDeviceToken stores an APNs device token for a given user+device.
// One entry per device — upserted by device_id when the APNs token changes.
type IOSDeviceToken struct {
	ID        string    `json:"id"         bson:"id"`
	UserID    string    `json:"user_id"    bson:"user_id"`
	DeviceID  string    `json:"device_id"  bson:"device_id"`
	Token     string    `json:"token"      bson:"token"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}
