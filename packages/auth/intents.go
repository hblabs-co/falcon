package auth

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/clientmeta"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// ── Throttle constant ───────────────────────────────────────────

// MaxIntentsPerHour is the rate cap used by the magic-link handler:
// 5 requests per email per hour is plenty for a real user (forgot
// the link, tried twice, opened in wrong device) without letting a
// bot run loose against Mailjet's quota. Tunable. Applies to every
// email — admins included (otherwise an attacker can burn Mailjet
// quota by hammering an admin address).
const MaxIntentsPerHour = 5

// Pagination defaults / caps for the admin /auth/intents listing.
// `defaultPageSize` matches what an admin UI grid typically shows
// per page; `maxPageSize` keeps a malicious / curious operator
// from materialising a million rows in a single response.
const (
	defaultPageSize = 100
	maxPageSize     = 500
)

// ── Helpers used by handleMagic ─────────────────────────────────

// recordIntent persists the magic-link attempt as an append-only
// row in auth_intents. Snapshot fields capture the state at request
// time (does this email already have a `users` doc? a CV?). Returns
// an error on insert failure — the magic-link handler must
// fail-closed because the throttle in step 3 counts these rows; a
// silent miss would let an attacker burst through (reports.md B14).
//
// `client` is the bag of HTTP request metadata produced by
// clientmeta.Capture at the call site — passed in so this helper
// stays HTTP-framework-agnostic (the caller adapts gin / fiber /
// whatever to clientmeta).
func recordIntent(ctx context.Context, email, deviceID, platform string, client clientmeta.ClientMeta) error {
	email = datasource.NormalizeEmail(email)
	snap := snapshotForIntent(ctx, email)
	intent := models.AuthIntent{
		ID:          gonanoid.Must(),
		Email:       email,
		RequestedAt: time.Now(),
		DeviceID:    deviceID,
		Platform:    platform,
		Client:      client,
		Context: models.AuthIntentContext{
			UserExistedAtRequest: snap.UserExisted,
			CVUploadedAtRequest:  snap.CVUploaded,
		},
	}
	if err := system.GetStorage().Insert(ctx, constants.MongoAuthIntentsCollection, intent); err != nil {
		return err
	}
	return nil
}

// recentIntentsCount returns how many auth_intents rows exist for
// the given email since `since`. Used by the throttle in the
// magic-link handler.
//
// Returns the underlying Mongo error so the caller can fail-closed:
// if Count fails we don't know how many recent intents exist and
// must NOT pretend the bucket is empty (that would disable the
// throttle exactly when an attacker is hammering Mongo).
func recentIntentsCount(ctx context.Context, email string, since time.Time) (int64, error) {
	return system.GetStorage().Count(ctx, constants.MongoAuthIntentsCollection, bson.M{
		"email":        datasource.NormalizeEmail(email),
		"requested_at": bson.M{"$gte": since},
	})
}

// ── Internal context helper ─────────────────────────────────────

// intentSnapshot captures the two booleans the AuthIntentContext
// snapshot needs. Returned together by snapshotForIntent so we
// only do one users lookup for the pair (the older split helpers
// each did their own GetByField, doubling round-trips).
type intentSnapshot struct {
	UserExisted bool
	CVUploaded  bool
}

// snapshotForIntent returns the per-email snapshot used inside
// AuthIntentContext. Errors are swallowed (zero-value returned)
// because this is best-effort context for an audit row — we don't
// want a transient Mongo blip to corrupt the row or block the
// magic-link flow.
func snapshotForIntent(ctx context.Context, email string) intentSnapshot {
	user, err := datasource.FindUserByEmail(ctx, email)
	if err != nil {
		return intentSnapshot{}
	}
	count, err := system.GetStorage().Count(ctx, constants.MongoCVsCollection, bson.M{
		"user_id": user.ID,
		"status":  bson.M{"$in": models.CVStatusesUsableBSON()},
	})
	return intentSnapshot{
		UserExisted: true,
		CVUploaded:  err == nil && count > 0,
	}
}

// ── Handler: GET /auth/intents ──────────────────────────────────

// AdminListIntents returns auth_intents rows filtered by query params.
// Pagination via page + page_size.
//
// Query params:
//
//	email     — exact match
//	ip        — exact match (matched against client.ip in the schema)
//	from      — RFC3339, requested_at >= from
//	to        — RFC3339, requested_at <= to
//	has_user  — "true" to keep only emails present in `users`,
//	            "false" to keep only emails NOT in `users` (post-
//	            filter in Go since this needs a join)
//	page      — 1-based, default 1
//	page_size — default 100, max 500
//
// Sorted by requested_at desc.
func AdminListIntents(c *gin.Context) {
	filter, ok := buildIntentFilter(c)
	if !ok {
		return // buildIntentFilter already wrote the 400 response
	}
	page, pageSize := parsePagination(c)

	var intents []models.AuthIntent
	total, err := system.GetStorage().FindPage(c.Request.Context(),
		constants.MongoAuthIntentsCollection,
		filter,
		"requested_at", true,
		page, pageSize, &intents,
	)
	if err != nil {
		logrus.Errorf(logPrefix+" list intents: %v", err)
		system.RespondInternal(c)
		return
	}

	// Optional has_user post-filter — done in Go because Mongo
	// doesn't have a cheap "email exists in users collection"
	// operator. Acceptable cost for an admin UI page (~100 docs).
	hasUser := c.Query("has_user")
	if hasUser == "true" || hasUser == "false" {
		intents = filterIntentsByUserPresence(c.Request.Context(), intents, hasUser == "true")
	}

	c.JSON(http.StatusOK, gin.H{
		"count":     len(intents),
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"intents":   intents,
	})
}

// buildIntentFilter extracts the per-query Mongo filter for
// listIntents from the gin.Context. Returns ok=false when a date
// param is malformed — in that case the response has already been
// written and the caller should return.
func buildIntentFilter(c *gin.Context) (bson.M, bool) {
	filter := bson.M{}
	if email := c.Query("email"); email != "" {
		filter["email"] = datasource.NormalizeEmail(email)
	}
	// IP lives nested under the `client` subdoc since the
	// clientmeta refactor. Same `NormalizeIP` primitive used at
	// write time (`Capture`) so the canonical form on both sides
	// matches — no false-empty results from `2001:DB8` vs
	// `2001:db8` (reports.md B21, N3). A malformed IP returns 400
	// instead of an empty result that looks like "no abuse".
	if ip := c.Query("ip"); ip != "" {
		canonical, ok := clientmeta.NormalizeIP(ip)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ip must be a valid IPv4/IPv6 address"})
			return nil, false
		}
		filter["client.ip"] = canonical
	}
	if from := c.Query("from"); from != "" {
		t, err := time.Parse(time.RFC3339, from)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "from must be RFC3339"})
			return nil, false
		}
		filter["requested_at"] = bson.M{"$gte": t}
	}
	if to := c.Query("to"); to != "" {
		t, err := time.Parse(time.RFC3339, to)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "to must be RFC3339"})
			return nil, false
		}
		// Merge with `from` if already present.
		if existing, ok := filter["requested_at"].(bson.M); ok {
			existing["$lte"] = t
		} else {
			filter["requested_at"] = bson.M{"$lte": t}
		}
	}
	return filter, true
}

// parsePagination reads `page` (1-based) and `page_size` from the
// query string, applying defaults and the maxPageSize cap.
func parsePagination(c *gin.Context) (page, pageSize int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultPageSize)))
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

// filterIntentsByUserPresence keeps only intents whose email
// presence in `users` matches wantPresent. One bulk fetch ($in)
// against `users` — cheap relative to per-row joins.
func filterIntentsByUserPresence(ctx context.Context, intents []models.AuthIntent, wantPresent bool) []models.AuthIntent {
	if len(intents) == 0 {
		return intents
	}
	emails := make([]string, 0, len(intents))
	seen := make(map[string]bool)
	for _, it := range intents {
		if !seen[it.Email] {
			seen[it.Email] = true
			emails = append(emails, it.Email)
		}
	}

	var users []models.User
	if err := system.GetStorage().GetMany(ctx,
		constants.MongoUsersCollection,
		bson.M{"email": bson.M{"$in": emails}},
		&users,
	); err != nil {
		logrus.Warnf(logPrefix+" intents has_user filter: load users failed: %v", err)
		return intents
	}
	present := make(map[string]bool, len(users))
	for _, u := range users {
		present[u.Email] = true
	}

	out := make([]models.AuthIntent, 0, len(intents))
	for _, it := range intents {
		if present[it.Email] == wantPresent {
			out = append(out, it)
		}
	}
	return out
}
