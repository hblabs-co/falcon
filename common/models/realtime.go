package models

import "time"

// Common realtime event types. Kept as constants so producers (clients) and
// consumers (falcon-realtime) agree on the vocabulary. New types can be added
// freely — the stats pipeline doesn't validate against this list.
const (
	// Connection lifecycle — covers the socket open/close but NOT "user was
	// in the app": use app_foregrounded / app_backgrounded for that.
	RealtimeEventSessionStarted = "session_started"

	// App visibility lifecycle.
	//  app_opened       = first .active of the process (cold launch).
	//  app_foregrounded = every .active AFTER the first (warm resume from
	//                     background, app switcher, lock). Keeps launches
	//                     and resumes separately measurable.
	//  app_backgrounded = iOS sent the app to background (home, switcher,
	//                     lock). Does NOT fire on kill — the server reaps
	//                     those via the read deadline.
	RealtimeEventAppOpened       = "app_opened"
	RealtimeEventAppForegrounded = "app_foregrounded"
	RealtimeEventAppBackgrounded = "app_backgrounded"

	RealtimeEventNotificationOpen = "notification_opened"
	RealtimeEventLiveActivityOpen = "live_activity_opened"
	RealtimeEventProjectViewed    = "project_viewed"
	RealtimeEventMatchViewed      = "match_viewed"
	RealtimeEventOriginalOpened   = "original_opened"
	RealtimeEventContactCalled    = "contact_called"
	RealtimeEventContactEmailed   = "contact_emailed"

	// Auth flow.
	RealtimeEventMagicLinkRequested = "magic_link_requested"

	// Server-emitted connection lifecycle. Authoritative source for
	// "was the user online?" — fires even when iOS kills the app
	// without firing willTerminate. device_offline always follows a
	// device_online for the same (device_id, connection); the pair
	// can be used to compute session duration.
	RealtimeEventDeviceOnline  = "device_online"
	RealtimeEventDeviceOffline = "device_offline"

	// Server-emitted auth transitions on an already-open socket. Triggered
	// by "user_bind" / "user_unbind" frames the client sends on login and
	// logout. Distinct from device_online/offline: the socket itself keeps
	// running — only the user_id bound to it changes.
	RealtimeEventUserBound   = "user_bound"
	RealtimeEventUserUnbound = "user_unbound"
)

// RealtimeEvent is the canonical record persisted in realtime_stats. A single
// shape for all event types keeps the pipeline simple — variance goes into
// Metadata, not new fields. One doc per event, append-only.
type RealtimeEvent struct {
	ID       string `bson:"id"        json:"id"`
	Event    string `bson:"event"     json:"event"`     // one of RealtimeEvent* consts above
	UserID   string `bson:"user_id"   json:"user_id"`   // empty for pre-login events (app_opened before auth)
	DeviceID string `bson:"device_id" json:"device_id"` // always set (from HMAC auth)
	Platform string `bson:"platform"  json:"platform"`  // "ios" | "android" | "web"

	// OS / OSVersion / AppVersion are sent on session_started; other events
	// can omit them. Kept at top level (not in Metadata) because they're
	// useful for aggregate queries (counts by iOS version, etc.).
	OS         string `bson:"os,omitempty"          json:"os,omitempty"`
	OSVersion  string `bson:"os_version,omitempty"  json:"os_version,omitempty"`
	AppVersion string `bson:"app_version,omitempty" json:"app_version,omitempty"`

	// IP is resolved server-side from the WebSocket request headers — never
	// trusted from the client payload. Empty when not available.
	IP string `bson:"ip,omitempty" json:"ip,omitempty"`

	// Metadata carries event-specific fields (project_id, match_id, etc.).
	// Kept as map[string]any so new event types don't require model changes.
	Metadata map[string]any `bson:"metadata,omitempty" json:"metadata,omitempty"`

	CreatedAt time.Time `bson:"created_at" json:"created_at"`
}
