package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// getStats returns aggregate counters for the Nest dashboard. Today
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
func getStats(c *gin.Context) {
	ctx := c.Request.Context()

	registered, err := system.GetStorage().Count(ctx, constants.MongoUsersCollection, bson.M{})
	if err != nil {
		respondInternal(c, "count users", err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": gin.H{
			"registered": registered,
		},
	})
}
