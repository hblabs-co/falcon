package models

import "time"

// IOSDeviceToken stores an APNs device token for a given user+device.
// One entry per device — upserted by device_id when the APNs token changes.
type IOSDeviceToken struct {
	ID     string `json:"id"         bson:"id"`
	UserID string `json:"user_id"    bson:"user_id"`
	// DeviceID is the keychain-persisted UUID unique per iOS install. One row
	// per device so a user with multiple devices gets notifications on each.
	// Per-device settings (like language) live in UserConfig keyed by device_id.
	DeviceID string `json:"device_id"  bson:"device_id"`
	Token    string `json:"token"      bson:"token"`
	// LiveActivityToken is the push-to-start token for the FalconMatchAttributes
	// activity type. iOS 17.2+ only — empty for older devices, in which case
	// signal falls back to the regular APNs push without a Live Activity.
	LiveActivityToken string `json:"live_activity_token,omitempty" bson:"live_activity_token,omitempty"`
	// LiveActivityUpdateToken is the per-activity update token iOS hands out
	// when an activity starts. As long as this is set, signal sends an
	// update-push (event: "update") to refresh the existing activity instead
	// of creating a new one. Cleared when iOS ends/dismisses the activity.
	LiveActivityUpdateToken string    `json:"live_activity_update_token,omitempty" bson:"live_activity_update_token,omitempty"`
	CreatedAt               time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt         time.Time `json:"updated_at" bson:"updated_at"`
}
