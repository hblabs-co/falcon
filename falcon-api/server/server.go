package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/system"
)

// RouteGroup is implemented by every feature package that wants to register
// HTTP routes. Mount is called once at startup with the root gin engine.
type RouteGroup interface {
	Mount(r *gin.Engine)
}

// Run starts the HTTP server with all provided route groups mounted.
func Run(groups ...RouteGroup) error {
	port := helpers.ReadEnvOptional("PORT", "8080")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger())
	r.SetTrustedProxies(nil)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for _, g := range groups {
		g.Mount(r)
	}

	return r.Run(":" + port)
}

// JWTMiddleware validates Bearer tokens signed with HS256 using the shared secret.
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
		if _, err := parser.Parse(raw, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		}); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Next()
	}
}

func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		logrus.WithFields(logrus.Fields{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"status": c.Writer.Status(),
		}).Info("request")
	}
}
