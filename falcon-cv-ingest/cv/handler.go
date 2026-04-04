package cv

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/system"
)

// routes registers all HTTP routes on r for svc.
func routes(r *gin.Engine, svc *Service) {
	secret := system.MustEnv("JWT_SECRET")

	r.GET("/health", Health)

	cvGroup := r.Group("/cv")
	cvGroup.POST("/prepare", handlePrepare(svc))
	cvGroup.POST("/:id/index", handleIndex(svc))
	cvGroup.GET("/:id", jwtMiddleware(secret), handleGet(svc))

}

func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handlePrepare godoc
// POST /cv/prepare
// Body: { "filename": "cv.docx" }
// Returns: { "cv_id", "upload_url", "expires_at" }
func handlePrepare(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Filename string `json:"filename" binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		result, err := svc.Prepare(c.Request.Context(), body.Filename)
		if err != nil {
			logrus.Errorf("prepare: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not prepare upload"})
			return
		}

		c.JSON(http.StatusCreated, result)
	}
}

// handleIndex godoc
// POST /cv/:id/index
// Body: { "email": "john@doe.com" }
// Triggers async processing of an uploaded CV.
func handleIndex(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		cvID := c.Param("id")

		var body struct {
			Email string `json:"email" binding:"required,email"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := svc.Index(cvID, body.Email); err != nil {
			logrus.Errorf("index %s: %v", cvID, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"cv_id": cvID, "status": "indexing"})
	}
}

// handleGet godoc
// GET /cv/:id
// Returns the current CV record from MongoDB.
func handleGet(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		cvID := c.Param("id")
		cv, err := svc.GetCV(c.Request.Context(), cvID)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "cv not found"})
			return
		}
		c.JSON(http.StatusOK, cv)
	}
}

// jwtMiddleware validates Bearer tokens signed with HS256 using the shared secret.
func jwtMiddleware(secret string) gin.HandlerFunc {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(auth, "Bearer ")

		_, err := parser.Parse(raw, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil {
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
