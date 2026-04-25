package users

import "github.com/gin-gonic/gin"

// Mount wires every user-centric admin route onto the
// already-authenticated group. Kept as the public entry point of
// this package so service.go reads as a flat route map.
//
// Resource map:
//
//	users     → search / detail
//	tokens    → magic links (test=true only)
//	sessions  → live JWT sessions (type=jwt, test=false)
//	devices   → APNs device tokens (read-only)
//	cv        → CV file download (presigned MinIO URL)
func Mount(g *gin.RouterGroup) {
	// Aggregated counters for the dashboard. Lives outside /users
	// so a future /stats namespace can grow with platform / signal
	// counters without coupling them to the user resource.
	g.GET("/stats", getStats)

	g.GET("/users/search", searchUsers)
	g.GET("/users/:id", getUser)

	g.GET("/users/:id/tokens", listUserTokens)
	g.POST("/users/:id/tokens", createUserToken)
	g.DELETE("/users/:id/tokens", deleteUserTokens)
	g.DELETE("/tokens/:id", deleteToken)

	g.GET("/users/:id/sessions", listUserSessions)
	g.DELETE("/users/:id/sessions", deleteUserSessions)
	g.DELETE("/sessions/:id", deleteSession)

	g.GET("/users/:id/devices", listUserDevices)
	g.GET("/users/:id/cv", downloadUserCV)

	g.GET("/users/:id/configs", listUserConfigs)
	g.GET("/users/:id/matches", listUserMatches)
	g.GET("/users/:id/realtime", listUserRealtime)

	// Match drill-down: nest's /match page calls this to build the
	// project detail view (raw + normalised + match) for any
	// (cv, project) pair without going through the user namespace.
	g.GET("/matches/:cv_id/:project_id", getMatchDetail)
}
