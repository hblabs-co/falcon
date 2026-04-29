package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"

	"context"
)

// ── Helper exposed to RequestMagicLink ───────────────────────────

// ActiveBlock returns the active AuthBlock for the given email, or
// nil if there isn't one. "Active" means the row exists and either
// has no expires_at (permanent) or expires_at is in the future.
// Mongo not finding the doc is the common case — return (nil, nil).
//
// Used by `falcon-api/auth/routes.go::handleMagic` to short-circuit
// magic-link requests for blocked emails. Fail-open: if Mongo
// returns an error other than ErrNoDocuments, the caller decides
// whether to allow the request through.
func ActiveBlock(ctx context.Context, email string) (*models.AuthBlock, error) {
	var block models.AuthBlock
	err := system.GetStorage().Get(ctx,
		constants.MongoAuthBlocksCollection,
		bson.M{"email": datasource.NormalizeEmail(email)},
		&block,
	)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !block.Active(time.Now()) {
		return nil, nil
	}
	return &block, nil
}

// ActiveBlocksByEmail is the bulk version of ActiveBlock — given a
// page of candidate emails, returns a set keyed by email of those
// whose block is currently active (no expires_at, or expires_at >
// now). One Mongo round-trip; lookup at the call site is O(1).
//
// The returned map is meant for "is this email blocked?" gates; it
// does NOT carry the AuthBlock body. Callers that need the reason/
// scope/expires_at should fall back to ActiveBlock for that single
// email — the bulk gate stays cheap.
//
// Caller policy on Mongo error: this function returns the error as-
// is. Reminder loops wrap this with fail-open + warn (silencing
// banned users on a transient blip is worse than briefly noop'ing
// the gate). Other callers may want strict failure — at the auth
// boundary `ActiveBlock` is fail-open by convention; here we leave
// the choice to the caller.
func ActiveBlocksByEmail(ctx context.Context, emails []string, now time.Time) (map[string]bool, error) {
	if len(emails) == 0 {
		return nil, nil
	}
	var rows []models.AuthBlock
	if err := system.GetStorage().GetMany(ctx,
		constants.MongoAuthBlocksCollection,
		bson.M{"email": bson.M{"$in": emails}},
		&rows,
	); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(rows))
	for _, r := range rows {
		if r.Active(now) {
			set[r.Email] = true
		}
	}
	return set, nil
}

// ── Handlers ─────────────────────────────────────────────────────

// AdminListBlocks returns every row in auth_blocks. Active and expired
// both come back — admin UI is the place to filter, not the API.
// Sort newest blocked first.
func AdminListBlocks(c *gin.Context) {
	var blocks []models.AuthBlock
	if err := system.GetStorage().GetMany(c.Request.Context(),
		constants.MongoAuthBlocksCollection,
		bson.M{},
		&blocks,
	); err != nil {
		logrus.Errorf(logPrefix+" list blocks: %v", err)
		system.RespondInternal(c)
		return
	}

	now := time.Now()
	type view struct {
		models.AuthBlock
		Active bool `json:"active"`
	}
	out := make([]view, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, view{AuthBlock: b, Active: b.Active(now)})
	}
	c.JSON(http.StatusOK, gin.H{"count": len(out), "blocks": out})
}

// AdminCreateBlock writes a new auth_blocks row. Body:
//
//	{
//	  "email":      "...",
//	  "reason":     "abuse" | "spam" | "manual",
//	  "scope":      "magic_link" | "global",
//	  "expires_at": "2026-12-31T..."  // optional, omit for permanent
//	}
//
// `scope: global` is accepted but only `magic_link` is enforced
// today — see notes/AUTH.md.
func AdminCreateBlock(c *gin.Context) {
	var body struct {
		Email     string                 `json:"email"      binding:"required,email"`
		Reason    models.AuthBlockReason `json:"reason"     binding:"required,oneof=abuse spam manual"`
		Scope     models.AuthBlockScope  `json:"scope"      binding:"required,oneof=magic_link global"`
		ExpiresAt *time.Time             `json:"expires_at"`
	}
	if !system.BindJSONOrAbort(c, &body) {
		return
	}
	body.Email = datasource.NormalizeEmail(body.Email)

	ctx := c.Request.Context()

	// Reuse the row's existing id when re-blocking the same email.
	// Without this, every re-block generates a fresh nanoid via
	// upsert-replace; an admin who cached the original id from a
	// prior response can no longer DELETE that row (reports.md
	// #10). BlockedAt is preserved on re-block to keep the
	// "first banned at" timestamp truthful — refresh ExpiresAt
	// and Reason/Scope as part of the update.
	id := gonanoid.Must()
	blockedAt := time.Now()
	var existing models.AuthBlock
	switch err := system.GetStorage().Get(ctx,
		constants.MongoAuthBlocksCollection,
		bson.M{"email": body.Email},
		&existing,
	); {
	case err == nil:
		id = existing.ID
		blockedAt = existing.BlockedAt
	case errors.Is(err, mongo.ErrNoDocuments):
		// new block — keep the freshly-generated id
	default:
		logrus.Errorf(logPrefix+" lookup existing block %s: %v", body.Email, err)
		system.RespondInternal(c)
		return
	}

	doc := models.AuthBlock{
		ID:        id,
		Email:     body.Email,
		BlockedAt: blockedAt,
		Reason:    body.Reason,
		Scope:     body.Scope,
		ExpiresAt: body.ExpiresAt,
	}

	// Set with email filter respects the unique (email) index —
	// upserts the row in place. Combined with the lookup above,
	// id stays stable across re-blocks.
	if err := system.GetStorage().Set(ctx,
		constants.MongoAuthBlocksCollection,
		bson.M{"email": body.Email},
		doc,
	); err != nil {
		logrus.Errorf(logPrefix+" create block %s: %v", body.Email, err)
		system.RespondInternal(c)
		return
	}

	logrus.Infof(logPrefix+" block created: email=%s reason=%s scope=%s",
		body.Email, body.Reason, body.Scope)
	c.JSON(http.StatusCreated, doc)
}

// AdminDeleteBlockById removes a block by its id (the AuthBlock.ID field).
// Re-running on an already-deleted id is a no-op (the DeleteMany
// matches nothing).
func AdminDeleteBlockById(c *gin.Context) {
	id, ok := system.RequireParam(c, "id")
	if !ok {
		return
	}
	if err := system.GetStorage().DeleteMany(c.Request.Context(),
		constants.MongoAuthBlocksCollection,
		bson.M{"id": id},
	); err != nil {
		logrus.Errorf(logPrefix+" delete block %s: %v", id, err)
		system.RespondInternal(c)
		return
	}
	logrus.Infof(logPrefix+" block deleted: id=%s", id)
	c.Status(http.StatusNoContent)
}
