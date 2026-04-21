package models

import "time"

// UserConfig stores a single named configuration value for a user on a specific
// platform, optionally scoped to a single device.
//
// When DeviceID is empty the config applies to all of the user's devices on that
// platform (user-wide default). When DeviceID is set, the config only applies to
// that specific device — used for settings that naturally differ per device,
// like the app language (a user may run iOS in English on one phone and German
// on another).
//
// Lookup fallback: device-specific → user-wide default. See the signal service's
// resolveDeviceLanguage for the canonical pattern.
type UserConfig struct {
	UserID    string    `json:"user_id"              bson:"user_id"`
	Platform  string    `json:"platform"             bson:"platform"`
	DeviceID  string    `json:"device_id,omitempty"  bson:"device_id,omitempty"`
	Name      string    `json:"name"                 bson:"name"`
	Value     any       `json:"value"                bson:"value"`
	UpdatedAt time.Time `json:"updated_at"           bson:"updated_at"`
}
