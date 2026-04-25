package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/ownhttp"
	"hblabs.co/falcon/packages/system"
)

// RouteGroup is implemented by every feature package that wants to register
// HTTP routes. Mount is called once at startup with the root gin engine.
type RouteGroup interface {
	Mount(r *gin.Engine)
}

// Run starts the HTTP server with all provided route groups mounted.
func Run(groups ...RouteGroup) error {
	port := environment.ReadOptional("PORT", "8080")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger())
	r.SetTrustedProxies(nil)

	// Match GET + HEAD: Nest's status poller probes with HEAD first
	// to avoid pulling the body. Gin doesn't auto-route HEAD onto a
	// GET handler the way net/http's mux does, so without HEAD here
	// every poll tick logs a spurious 404.
	r.Match([]string{http.MethodGet, http.MethodHead}, "/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for _, g := range groups {
		g.Mount(r)
	}

	return r.Run(":" + port)
}

// JWTMiddleware validates Bearer tokens signed with HS256, checks they haven't
// been revoked in the tokens collection, and injects "user_id" and "email" into
// the gin context for downstream handlers.
func JWTMiddleware() gin.HandlerFunc {
	secret := system.MustEnv("JWT_SECRET")
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(auth, "Bearer ")
		tok, err := parser.Parse(raw, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := tok.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		// Check revocation via jti.
		if jti, _ := claims["jti"].(string); jti != "" {
			var stored models.Token
			if err := system.GetStorage().GetById(c.Request.Context(), constants.MongoTokensCollection, jti, &stored); err == nil {
				if stored.Revoked {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token revoked"})
					return
				}
			}
		}

		// Inject into context for downstream handlers.
		if sub, _ := claims["sub"].(string); sub != "" {
			c.Set("user_id", sub)
		}
		if email, _ := claims["email"].(string); email != "" {
			c.Set("email", email)
		}

		c.Next()
	}
}

// ginLogger delegates to the shared `ownhttp.LogRequest` so Gin-based
// services emit the same log shape as net/http services (landing,
// admin). Records start time before `c.Next()` so the duration field
// reflects the full handler chain, including middleware.
func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		ownhttp.LogRequest(
			c.Request.Method,
			c.Request.URL.Path,
			c.Writer.Status(),
			time.Since(start),
		)
	}
}
