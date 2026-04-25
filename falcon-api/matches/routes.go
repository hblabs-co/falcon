package matches

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/helpers"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

const pageSize = 20

// Routes implements server.RouteGroup for match feed endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/matches", server.JWTMiddleware())
	g.GET("", handleListMatches)
	g.PATCH("/viewed", handleMarkViewed)
}

// handleListMatches returns the authenticated user's match results, ordered by
// scored_at desc (chronological — when each match was produced by the LLM).
// All matches are returned regardless of score so the user can also see weak
// candidates with the "not_suitable" label and decide for themselves.
//
// TODO: support `?sort=` query param. Options:
//   - sort=score          → score desc (best matches first)
//   - sort=display        → JOIN with projects_normalized, sort by project freshness
//     (requires mongo aggregation pipeline)
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

	// Canonical "what the user can see" predicate — shared with
	// falcon-signal so Live Activity counters always agree with the
	// list. Anything more subtle than the defaults goes through
	// `helpers.VisibleMatchFilter` so both sides stay in sync.
	listFilter := helpers.VisibleMatchFilter(uid)

	// Optional "unread only" client filter. `viewed: {$ne: true}`
	// matches false, null, and missing — covers pre-feature docs that
	// never got the field written, not just explicit `viewed: false`.
	if c.Query("only_unread") == "true" {
		listFilter["viewed"] = bson.M{"$ne": true}
	}

	var docs []models.MatchResultEvent
	total, err := store.FindPage(ctx, constants.MongoMatchResultsCollection,
		listFilter,
		"scored_at", true,
		page, pageSize, &docs)
	if err != nil {
		logrus.Errorf("list matches user=%s page=%d: %v", uid, page, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch matches"})
		return
	}
	// Empty result must serialise as `[]`, not `null` — the iOS decoder
	// treats a null `data` field as "missing" and renders a full-screen
	// error instead of the empty-state list with filters intact.
	if docs == nil {
		docs = []models.MatchResultEvent{}
	}

	// Count unviewed matches for this user across ALL pages — drives the
	// tab-icon badge. Missing `viewed` field counts as unread (pre-
	// feature docs). Cheap on an indexed collection and avoids a second
	// round-trip from the client. Starts from the same canonical
	// visible-match filter so the badge never counts matches the user
	// can't see.
	unreadFilter := helpers.VisibleMatchFilter(uid)
	unreadFilter["$or"] = []bson.M{
		{"viewed": bson.M{"$exists": false}},
		{"viewed": false},
	}
	unread, err := store.Count(ctx, constants.MongoMatchResultsCollection, unreadFilter)
	if err != nil {
		logrus.Warnf("count unread matches user=%s: %v", uid, err)
		unread = 0
	}

	c.JSON(http.StatusOK, gin.H{
		"data":         docs,
		"pagination":   server.Paginate(page, pageSize, total),
		"unread_count": unread,
	})
}

// handleMarkViewed flips the `viewed` flag to true for a single match.
// Body: {"project_id": "...", "cv_id": "..."}. Scoped to the authenticated
// user so one user can't mark another's match as viewed. Idempotent —
// re-marking is a no-op.
func handleMarkViewed(c *gin.Context) {
	userID, _ := c.Get("user_id")
	if userID == nil || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	uid := userID.(string)

	var body struct {
		ProjectID string `json:"project_id" binding:"required"`
		CVID      string `json:"cv_id"      binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	store := system.GetStorage()

	// (user_id, project_id, cv_id) is unique per match → UpdateOne, not
	// BulkUpdate. Non-upserting so a wrong id-pair doesn't create a
	// phantom doc.
	if _, err := store.UpdateOne(ctx, constants.MongoMatchResultsCollection,
		bson.M{"user_id": uid, "project_id": body.ProjectID, "cv_id": body.CVID},
		bson.M{"$set": bson.M{"viewed": true}},
	); err != nil {
		logrus.Errorf("mark viewed user=%s project=%s cv=%s: %v", uid, body.ProjectID, body.CVID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to mark viewed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
