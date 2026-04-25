package admin

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/admin/issues"
	"hblabs.co/falcon/admin/users"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// Handler returns the Gin engine with every admin endpoint wired up.
// Kept in one place so the full API surface of the admin service reads
// top-to-bottom in this function. The user-centric subset (search +
// per-user views + sessions + CV download) lives in the `users`
// package and is mounted via users.Mount.
func Handler() http.Handler {
	r := gin.New()
	r.Use(gin.Recovery())

	// Public — used by k8s-style health checks, a `watch curl` loop,
	// or Nest's status poller. Match GET + HEAD: Gin doesn't auto-
	// route HEAD onto a GET handler, so without HEAD here every
	// HEAD probe logs a spurious 404.
	r.Match([]string{http.MethodGet, http.MethodHead}, "/healthz",
		func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// Everything past this group goes through the bearer check.
	admin := r.Group("/", requireBearer())
	{
		admin.POST("/test-link", createTestLink)
		admin.GET("/test-links", listTestLinks)
		admin.DELETE("/test-link/:id", deleteOneTestLink)
		admin.DELETE("/test-links", purgeAllTestLinks)
		// User-centric admin (Nest's /users UI): autocomplete, magic
		// links, JWT sessions, devices, CV download.
		users.Mount(admin)
		issues.Mount(admin)
	}

	return r
}

// requireBearer compares the request's `Authorization: Bearer <x>`
// header against ADMIN_TOKEN. If the env var is unset we let all
// requests through — convenient for a localhost-only smoke test, but
// we log a warning so nobody ships the service with its door open.
func requireBearer() gin.HandlerFunc {
	want := os.Getenv("ADMIN_TOKEN")
	if want == "" {
		logrus.Warn("[admin] ADMIN_TOKEN not set — admin endpoints are UNAUTHENTICATED")
	}
	return func(c *gin.Context) {
		if want == "" {
			c.Next()
			return
		}
		got := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if got != want {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}

// createTestLink generates a new long-lived multi-use magic token
// and returns the deep-link URL. Body:
//
//	{ "email": "reviewer@apple.com", "device_id": "optional" }
//
// device_id is optional — if absent we mint a random `test-<nanoid>`
// placeholder so the token model's Validate() passes without tying
// the link to a specific phone. user_id is left empty here; use
// POST /users/:id/tokens (users package) to stamp it.
func createTestLink(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required,email"`
		DeviceID string `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.DeviceID == "" {
		body.DeviceID = "test-" + gonanoid.Must(12)
	}

	doc, link, err := users.MintTestLink(c.Request.Context(), body.Email, "", body.DeviceID)
	if err != nil {
		if vErr, ok := err.(users.ValidationError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": vErr.Error()})
			return
		}
		logrus.Errorf("save test token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	logrus.Infof("[admin] created test link for %s (id=%s, expires=%s)",
		body.Email, doc.ID, doc.ExpiresAt.Format(time.RFC3339))

	c.JSON(http.StatusCreated, gin.H{
		"id":         doc.ID,
		"email":      doc.Email,
		"device_id":  doc.DeviceID,
		"link":       link,
		"expires_at": doc.ExpiresAt.Format(time.RFC3339),
	})
}

// listTestLinks returns every token with `test: true`. Hashes are
// never leaked — only metadata. Re-issuing a link means creating a
// new one. Kept for backward compat with the original CLI flow; the
// new UI uses the per-user endpoints in the users package instead.
func listTestLinks(c *gin.Context) {
	var tokens []models.Token
	if err := system.GetStorage().GetMany(
		c.Request.Context(),
		constants.MongoTokensCollection,
		bson.M{"test": true},
		&tokens,
	); err != nil {
		logrus.Errorf("list test tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	type view struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		DeviceID  string    `json:"device_id"`
		ExpiresAt time.Time `json:"expires_at"`
		CreatedAt time.Time `json:"created_at"`
		UserID    string    `json:"user_id,omitempty"`
	}
	out := make([]view, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, view{t.ID, t.Email, t.DeviceID, t.ExpiresAt, t.CreatedAt, t.UserID})
	}
	c.JSON(http.StatusOK, gin.H{"count": len(out), "tokens": out})
}

// deleteOneTestLink removes a single test token by its id (the
// Token.ID field — NOT the raw magic link). Safety rail: only
// deletes rows that have `test: true` so a stray id can't nuke a
// real user's JWT.
func deleteOneTestLink(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	if err := system.GetStorage().DeleteMany(
		c.Request.Context(),
		constants.MongoTokensCollection,
		bson.M{"id": id, "test": true},
	); err != nil {
		logrus.Errorf("delete test token %s: %v", id, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	logrus.Infof("[admin] deleted test token %s", id)
	c.Status(http.StatusNoContent)
}

// purgeAllTestLinks wipes every token with `test: true`. Use after
// App Store review wraps up so leftover long-lived links don't
// linger for another ~29 days before the TTL catches up.
func purgeAllTestLinks(c *gin.Context) {
	if err := system.GetStorage().DeleteMany(
		c.Request.Context(),
		constants.MongoTokensCollection,
		bson.M{"test": true},
	); err != nil {
		logrus.Errorf("purge test tokens: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	logrus.Infof("[admin] purged all test tokens")
	c.Status(http.StatusNoContent)
}
