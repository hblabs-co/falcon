package admin

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/admin/auth"
	"hblabs.co/falcon/admin/issues"
	"hblabs.co/falcon/admin/matches"
	"hblabs.co/falcon/admin/signal"
	"hblabs.co/falcon/admin/stats"
	"hblabs.co/falcon/admin/users"
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
	admin := r.Group("/admin", requireBearer())

	// One-shot scout trigger — kicks today's scrape via NATS.
	// POST so a stray Safari paste / link prefetch can't fire it.
	admin.POST("/scout/scan-today", triggerScanToday)

	// Signal pipeline test triggers (test-alert / test-last-match
	// / test-push / test-email) — lives in `falcon-admin/signal/`.
	signal.Mount(admin)

	// Auth surface — every route under `/auth/...` (blocks,
	// intents, tokens, sessions). Lives in `falcon-admin/auth/`
	// so service.go stays a flat router map; handlers live in
	// `packages/auth`. See AUTH.md.
	auth.Mount(admin)

	// Dashboard counters consumed by Nest.
	stats.Mount(admin)

	// Match drill-down (raw + normalised + match for any
	// cv/project pair). Per-user listing stays under users.
	matches.Mount(admin)

	// User-centric admin (Nest's /users UI): autocomplete,
	// devices, CV download.
	users.Mount(admin)
	issues.Mount(admin)

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
