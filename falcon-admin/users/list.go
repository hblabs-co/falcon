package users

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// listUsers returns a paginated, recency-sorted slice of every user.
// Used by nest's /users page when no specific user is selected — the
// operator scrolls the list, clicks a row, and lands on the per-user
// view via the existing /users/:id endpoint.
//
// Sort: created_at desc (most recent signups at the top — the usual
// "what's been happening" view).
//
// Each row carries enough metadata to recognise the user without a
// follow-up query: email + name (from the normalized CV when
// available) + joined_at + has_cv.
//
// Query params:
//
//	page       1-indexed (default 1)
//	page_size  default 25, max 100
func listUsers(c *gin.Context) {
	ctx := c.Request.Context()

	page := parsePagination(c.Query("page"), 1, 10000)
	pageSize := parsePagination(c.Query("page_size"), 25, 100)

	var rows []models.User
	total, err := system.GetStorage().FindPage(ctx,
		constants.MongoUsersCollection,
		bson.M{},
		"created_at", true,
		page, pageSize, &rows,
	)
	if err != nil {
		respondInternal(c, "list users", err)
		return
	}

	// Enrich the rendered rows with the CV-derived name and has_cv
	// flag. One batched fetch over `cvs` keyed on user_id rather
	// than N per-row lookups.
	ids := make([]string, len(rows))
	for i, u := range rows {
		ids[i] = u.ID
	}
	cvByUser := map[string]models.PersistedCV{}
	if len(ids) > 0 {
		var cvs []models.PersistedCV
		if err := system.GetStorage().GetManyByField(ctx,
			constants.MongoCVsCollection, "user_id", ids, &cvs); err == nil {
			for _, cv := range cvs {
				cvByUser[cv.UserID] = cv
			}
		}
	}

	type listRow struct {
		userView
		HasCV bool `json:"has_cv"`
	}

	out := make([]listRow, 0, len(rows))
	for _, u := range rows {
		row := listRow{userView: userView{
			UserID:   u.ID,
			Email:    u.Email,
			JoinedAt: u.CreatedAt,
		}}
		if cv, ok := cvByUser[u.ID]; ok {
			row.FirstName, row.LastName = pickName(cv.Normalized)
			row.HasCV = cv.MinioBucket != "" && cv.MinioKey != ""
		}
		out = append(out, row)
	}

	c.JSON(http.StatusOK, gin.H{
		"users":     out,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// parsePagination is a small helper local to this file (and not the
// search.go parseLimit) because the semantics differ slightly: here
// 0/negative falls back to the default rather than being treated as
// "use full default" — keeps the API consistent with /issues which
// uses the same shape.
func parsePagination(raw string, def, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
