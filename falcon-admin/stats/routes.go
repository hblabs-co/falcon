// Package stats in falcon-admin owns the dashboard counters
// surface — one endpoint today (/stats with user counts), designed
// to grow into a single payload that populates every Nest dashboard
// badge so the UI fetches one response instead of fanning out.
//
// Mirrors `falcon-admin/auth/`, `falcon-admin/signal/`,
// `falcon-admin/matches/`, etc. — each admin domain owns its own
// Mount() so service.go stays a flat route map.
package stats

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// Mount wires the stats admin route(s) onto the already-
// authenticated admin group.
func Mount(parent *gin.RouterGroup) {
	statsGroup := parent.Group("/stats")
	statsGroup.GET("", GetStats)
}

// GetStats returns aggregate counters for the Nest dashboard. Today
// just user counts; designed to grow into a single stats endpoint
// (signals, devices, magic-link health, etc.) so the dashboard can
// fetch one payload to populate every badge instead of fanning out
// across resources.
//
// Shape:
//
//	{
//	  "users": { "registered": 123 }
//	}
//
// "Registered" is `users.count()` — every doc in the users
// collection. Anonymous-user accounting (devices that never claimed
// an email) lands here once the data model nails down where those
// are persisted.
func GetStats(c *gin.Context) {
	ctx := c.Request.Context()

	registered, err := system.GetStorage().Count(ctx, constants.MongoUsersCollection, bson.M{})
	if err != nil {
		logrus.Errorf("[admin] count users: %v", err)
		system.RespondInternal(c)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"registered": registered,
		},
	})
}
