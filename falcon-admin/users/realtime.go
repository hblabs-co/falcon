package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// listUserRealtime returns the user's recent realtime_stats events
// paginated, newest first. The full RealtimeEvent shape (Metadata
// included) is returned as-is — the UI summarises it, but having
// the raw map available lets the admin expand the row for
// inspection without another round-trip.
func listUserRealtime(c *gin.Context) {
	id := c.Param("id")
	if _, ok := loadUserOr404(c, id); !ok {
		return
	}

	page := parseLimit(c.Query("page"), 1, 10000)
	pageSize := parseLimit(c.Query("page_size"), 25, 100)

	var events []models.RealtimeEvent
	total, err := system.GetStorage().FindPage(c.Request.Context(),
		constants.MongoRealtimeStatsCollection,
		bson.M{"user_id": id}, "created_at", true, page, pageSize, &events)
	if err != nil {
		respondInternal(c, "list user realtime events", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"events":    events,
	})
}
