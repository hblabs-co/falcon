package users

import "github.com/gin-gonic/gin"

// Mount wires every user-centric admin route onto the
// already-authenticated group. Kept as the public entry point of
// this package so service.go reads as a flat route map.
//
// Resource map:
//
//	users     → search / detail / per-user views
//	devices   → APNs device tokens (read-only)
//	cv        → CV file download (presigned MinIO URL)
//
// Other domains live in their own admin modules:
//	auth/    → magic-link / session CRUD (packages/auth handlers)
//	matches/ → match drill-down (cv,project pair)
//	stats/   → dashboard counters
//	signal/  → pipeline test triggers
//	issues/  → issue triage
func Mount(g *gin.RouterGroup) {
	g.GET("/users/search", searchUsers)
	// Recency-sorted paginated list. Powers nest's "browse all
	// users" view when no specific user is being inspected.
	g.GET("/users", listUsers)
	g.GET("/users/:id", getUser)

	g.GET("/users/:id/devices", listUserDevices)
	g.GET("/users/:id/cv", downloadUserCV)

	g.GET("/users/:id/configs", listUserConfigs)
	g.GET("/users/:id/matches", listUserMatches)
	g.GET("/users/:id/realtime", listUserRealtime)
}
