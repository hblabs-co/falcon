package users

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/admin/matches"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// listUserMatches returns match_results for a user, paginated and
// sorted by scored_at desc. The wire shape (`matches.MatchView`) is
// shared with the drill-down endpoint in `falcon-admin/matches/` so
// list and detail render identically.
//
// Query params:
//
//   - min_score (float)  → `score >= min_score` Mongo filter.
//   - viewed   ("false") → only rows where viewed != true (unread).
//
// Both filters are applied at the database layer so totals + pages
// track them accurately — flipping the toggle in the UI fetches a
// fresh page instead of just hiding rows on the client.
func listUserMatches(c *gin.Context) {
	id := c.Param("id")
	if _, ok := loadUserOr404(c, id); !ok {
		return
	}

	page := parseLimit(c.Query("page"), 1, 10000)
	pageSize := parseLimit(c.Query("page_size"), 20, 100)

	filter := bson.M{"user_id": id}
	if raw := c.Query("min_score"); raw != "" {
		if v, err := strconv.ParseFloat(raw, 32); err == nil && v > 0 {
			filter["score"] = bson.M{"$gte": float32(v)}
		}
	}
	// `$ne: true` covers both viewed=false and rows where the field
	// is missing entirely (legacy data, default Go zero-value).
	if c.Query("viewed") == "false" {
		filter["viewed"] = bson.M{"$ne": true}
	}

	var rows []models.MatchResultEvent
	total, err := system.GetStorage().FindPage(c.Request.Context(),
		constants.MongoMatchResultsCollection,
		filter, "scored_at", true, page, pageSize, &rows)
	if err != nil {
		respondInternal(c, "list user matches", err)
		return
	}

	out := make([]matches.MatchView, 0, len(rows))
	for _, m := range rows {
		out = append(out, matches.ToMatchView(m))
	}
	c.JSON(http.StatusOK, gin.H{
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"matches":   out,
	})
}
