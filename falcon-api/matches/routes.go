package matches

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

const pageSize = 20

// Routes implements server.RouteGroup for match feed endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/matches", server.JWTMiddleware())
	g.GET("", handleListMatches)
}

// handleListMatches returns the authenticated user's match results, ordered by
// scored_at desc (chronological — when each match was produced by the LLM).
// All matches are returned regardless of score so the user can also see weak
// candidates with the "not_suitable" label and decide for themselves.
//
// TODO: support `?sort=` query param. Options:
//   - sort=score          → score desc (best matches first)
//   - sort=display        → JOIN with projects_normalized, sort by project freshness
//                           (requires mongo aggregation pipeline)
//
// Default stays scored_at desc which gives a "newest match for me" feed.
func handleListMatches(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid := userID.(string)

	page := server.ParsePage(c)

	ctx := c.Request.Context()
	store := system.GetStorage()

	var docs []models.MatchResultEvent
	total, err := store.FindPage(ctx, constants.MongoMatchResultsCollection,
		bson.M{
			"user_id":  uid,
			"platform": bson.M{"$ne": "freelance.de"},
		},
		"scored_at", true,
		page, pageSize, &docs)
	if err != nil {
		logrus.Errorf("list matches user=%s page=%d: %v", uid, page, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch matches"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":       docs,
		"pagination": server.Paginate(page, pageSize, total),
	})
}
