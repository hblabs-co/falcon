// Package auth in falcon-admin is the mount surface for every
// auth-domain admin route — blocks, intents, magic-link tokens,
// JWT sessions. All handlers live in `packages/auth`; this module
// only wires them onto the admin's bearer-protected gin group.
//
// Mirrors `falcon-admin/users/` and `falcon-admin/issues/` —
// each admin domain owns its own Mount() so service.go stays a
// flat, scannable route map.
//
// Every path is rooted at `/auth/...` so the admin URL surface
// reflects the domain layout: any URL the operator hits that
// touches authentication, sessions, or audit data starts with
// `/admin/auth/`. Earlier shapes scattered tokens under
// `/users/:id/tokens` and sessions under `/sessions/:id`; those
// were migrated under `/auth/tokens/...` and `/auth/sessions/...`
// to consolidate.
package auth

import (
	"github.com/gin-gonic/gin"
	pkgauth "hblabs.co/falcon/packages/auth"
)

// Mount wires every auth-domain admin route onto the
// already-authenticated group. Caller (`falcon-admin/admin/service.go`)
// is responsible for the bearer-token middleware.
func Mount(parent *gin.RouterGroup) {

	authGroup := parent.Group("/auth")

	// auth_blocks — admin manages emails blocked from requesting
	// magic links. handleMagic reads this collection on every
	// request to short-circuit blocked emails (fail-open on Mongo
	// error). See packages/auth/blocks.go.
	authGroup.GET("/blocks", pkgauth.AdminListBlocks)
	authGroup.POST("/blocks", pkgauth.AdminCreateBlock)
	authGroup.DELETE("/blocks/:id", pkgauth.AdminDeleteBlockById)

	// auth_intents — append-only audit log of every POST /auth/magic.
	// Filterable by email, IP, time window, has_user. See
	// packages/auth/intents.go.
	authGroup.GET("/intents", pkgauth.AdminListIntents)

	// auth_tokens — magic-link CRUD. Per-user CRUD is the modern
	// path (POSTs stamp user_id); the global delete-by-id sits
	// alongside for revoking a single token regardless of user.
	authGroup.GET("/tokens/user/:id", pkgauth.AdminListTokensByUserId)
	authGroup.POST("/tokens/user/:id", pkgauth.AdminCreateTokenForUserId)
	authGroup.DELETE("/tokens/user/:id", pkgauth.AdminDeleteTokensByUserId)
	authGroup.DELETE("/tokens/:id", pkgauth.AdminDeleteTokenById)

	// Legacy global test-link CRUD — predates user_id stamping.
	// Modern path is `/auth/tokens/user/:id`, but the original CLI
	// flow + the App Store reviewer issuance still hit these.
	// `DELETE /auth/test-link/:id` reuses `AdminDeleteTokenById` (same
	// filter `{id, test:true}` as the modern path — was a clone).
	authGroup.GET("/test-links", pkgauth.AdminListTestLinks)
	authGroup.POST("/test-link", pkgauth.AdminCreateTestLink)
	authGroup.DELETE("/test-link/:id", pkgauth.AdminDeleteTokenById)
	authGroup.DELETE("/test-links", pkgauth.AdminPurgeTestLinks)

	// auth_tokens (type=jwt) — live mobile sessions. Listing /
	// revoking is split from the magic-link CRUD so a stray click
	// in one UI can't accidentally kill data in the other.
	authGroup.GET("/sessions/user/:id", pkgauth.AdminListSessionsByUserId)
	authGroup.DELETE("/sessions/user/:id", pkgauth.AdminDeleteSessionsByUserId)
	authGroup.DELETE("/sessions/:id", pkgauth.AdminDeleteSessionById)
}
