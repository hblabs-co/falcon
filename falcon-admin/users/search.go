package users

import (
	"context"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

const (
	// Search caps — small enough that "match anything" can't pull
	// the whole collection back into memory, big enough to cover
	// the usual "I typed two letters" shape.
	defaultSearchLimit = 10
	maxSearchLimit     = 50
	minQueryLen        = 2
)

// searchUsers does a prefix search over `users.email` and over the
// six trilingual name fields on the normalized CV, then merges and
// enriches so the response always carries email + name + joined_at
// when known.
//
// Two queries (not an aggregation) on purpose: the data set is small
// and the merge is trivial in Go — clearer than a $lookup pipeline.
func searchUsers(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if len(q) < minQueryLen {
		c.JSON(http.StatusOK, gin.H{"results": []userView{}})
		return
	}
	limit := parseLimit(c.Query("limit"), defaultSearchLimit, maxSearchLimit)
	rx := prefixRegex(q)

	ctx := c.Request.Context()

	emailUsers, err := findUsersByEmailRegex(ctx, rx, limit)
	if err != nil {
		respondInternal(c, "search users by email", err)
		return
	}
	nameCVs, err := findCVsByNameRegex(ctx, rx, limit)
	if err != nil {
		respondInternal(c, "search cvs by name", err)
		return
	}

	merged := mergeSearchHits(emailUsers, nameCVs)
	enrichSearchHits(ctx, merged)

	out := make([]userView, 0, len(merged))
	for _, v := range merged {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].JoinedAt.After(out[j].JoinedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	c.JSON(http.StatusOK, gin.H{"results": out})
}

// prefixRegex builds a case-insensitive `^q` filter, escaping any
// regex metacharacters in the input.
func prefixRegex(q string) bson.M {
	return bson.M{"$regex": "^" + regexp.QuoteMeta(q), "$options": "i"}
}

func parseLimit(raw string, def, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func findUsersByEmailRegex(ctx context.Context, rx bson.M, limit int) ([]models.User, error) {
	var out []models.User
	_, err := system.GetStorage().FindPage(ctx, constants.MongoUsersCollection,
		bson.M{"email": rx}, "created_at", true, 1, limit, &out)
	return out, err
}

func findCVsByNameRegex(ctx context.Context, rx bson.M, limit int) ([]models.PersistedCV, error) {
	filter := bson.M{"$or": []bson.M{
		{"normalized.en.first_name": rx},
		{"normalized.en.last_name": rx},
		{"normalized.es.first_name": rx},
		{"normalized.es.last_name": rx},
		{"normalized.de.first_name": rx},
		{"normalized.de.last_name": rx},
	}}
	var out []models.PersistedCV
	_, err := system.GetStorage().FindPage(ctx, constants.MongoCVsCollection,
		filter, "created_at", true, 1, limit, &out)
	return out, err
}

// mergeSearchHits builds the dedup'd map keyed by user_id, seeded
// from both the email and name hits — whichever fields were present
// on the source row carry over directly.
func mergeSearchHits(emailUsers []models.User, nameCVs []models.PersistedCV) map[string]userView {
	out := map[string]userView{}
	for _, u := range emailUsers {
		out[u.ID] = userView{UserID: u.ID, Email: u.Email, JoinedAt: u.CreatedAt}
	}
	for _, cv := range nameCVs {
		if cv.UserID == "" {
			continue
		}
		v := out[cv.UserID]
		v.UserID = cv.UserID
		v.FirstName, v.LastName = pickName(cv.Normalized)
		out[cv.UserID] = v
	}
	return out
}

// enrichSearchHits fills in whatever the initial queries didn't see.
// A name-only hit doesn't carry email/joined_at; an email-only hit
// doesn't carry the name. One batched fetch per missing dimension.
func enrichSearchHits(ctx context.Context, merged map[string]userView) {
	var needUser, needCV []string
	for id, v := range merged {
		if v.Email == "" {
			needUser = append(needUser, id)
		}
		if v.FirstName == "" && v.LastName == "" {
			needCV = append(needCV, id)
		}
	}
	if len(needUser) > 0 {
		var users []models.User
		if err := system.GetStorage().GetManyByField(ctx,
			constants.MongoUsersCollection, "id", needUser, &users); err == nil {
			for _, u := range users {
				v := merged[u.ID]
				v.Email = u.Email
				if v.JoinedAt.IsZero() {
					v.JoinedAt = u.CreatedAt
				}
				merged[u.ID] = v
			}
		}
	}
	if len(needCV) > 0 {
		var cvs []models.PersistedCV
		if err := system.GetStorage().GetManyByField(ctx,
			constants.MongoCVsCollection, "user_id", needCV, &cvs); err == nil {
			for _, cv := range cvs {
				v, ok := merged[cv.UserID]
				if !ok {
					continue
				}
				if v.FirstName == "" && v.LastName == "" {
					v.FirstName, v.LastName = pickName(cv.Normalized)
					merged[cv.UserID] = v
				}
			}
		}
	}
}

// pickName returns the first non-empty (first, last) pair across the
// three CV languages. en > es > de — purely a tie-break, all three
// are fine since the normalizer fills them all.
func pickName(n *models.NormalizedCV) (string, string) {
	if n == nil {
		return "", ""
	}
	for _, lang := range []models.NormalizedCVLang{n.En, n.Es, n.De} {
		if lang.FirstName != "" || lang.LastName != "" {
			return lang.FirstName, lang.LastName
		}
	}
	return "", ""
}
