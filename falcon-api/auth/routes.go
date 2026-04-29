// Package auth in falcon-api is a thin Routes wrapper now. All
// the auth logic — handlers, helpers, the public unsubscribe page,
// the magic-link / verify flow, the JWT helpers — lives in
// `packages/auth`. This file only adapts that package to the
// falcon-api server.RouteGroup interface so the existing
// registration in main.go (`server.NewModule(auth.Routes{}, ...)`)
// keeps working without churn.
//
// To touch real auth logic, edit packages/auth.
package auth

import (
	"github.com/gin-gonic/gin"
	pkgauth "hblabs.co/falcon/packages/auth"
)

// Routes implements server.RouteGroup for the auth subsystem.
type Routes struct{}

// Mount registers every public auth endpoint on the engine root.
func (Routes) Mount(r *gin.Engine) {
	r.POST("/auth/magic", pkgauth.RequestMagicLink)
	r.GET("/auth/verify", pkgauth.VerifyMagicLink)
	r.GET("/unsubscribe", pkgauth.Unsubscribe)
}
