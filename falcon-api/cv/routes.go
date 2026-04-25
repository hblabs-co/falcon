package cv

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

const prepareTimeout = 10 * time.Second

// Routes implements server.RouteGroup for CV endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/cv")
	g.POST("/prepare", handlePrepare)
	g.POST("/:id/index", handleIndex)
	g.GET("/:id", handleGet)
}

// handlePrepare godoc
// POST /cv/prepare
// Body: { "filename": "cv.docx" }
// Returns: { "cv_id", "upload_url", "expires_at" }
// Calls falcon-storage via NATS core request/reply (synchronous).
func handlePrepare(c *gin.Context) {
	var body struct {
		Filename string `json:"filename" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req := models.CVPrepareRequestedEvent{
		RequestID: gonanoid.Must(),
		Filename:  body.Filename,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), prepareTimeout)
	defer cancel()

	var result models.CVPreparedEvent
	if err := system.Request(ctx, constants.SubjectCVPrepareRequested, req, &result); err != nil {
		logrus.Errorf("cv prepare request: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage service unavailable"})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// handleIndex godoc
// POST /cv/:id/index
// Body: { "email": "john@doe.com" }
// Publishes cv.index.requested to NATS (async).
func handleIndex(c *gin.Context) {
	cvID := c.Param("id")

	var body struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	evt := models.CVIndexRequestedEvent{
		CVID:  cvID,
		Email: body.Email,
	}
	if err := system.Publish(c.Request.Context(), constants.SubjectCVIndexRequested, evt); err != nil {
		logrus.Errorf("publish cv.index.requested: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue index request"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"cv_id": cvID, "status": "indexing"})
}

// handleGet godoc
// GET /cv/:id
// Returns the current CV record from MongoDB.
func handleGet(c *gin.Context) {
	cvID := c.Param("id")

	var cv models.PersistedCV
	if err := system.GetStorage().GetById(c.Request.Context(), constants.MongoCVsCollection, cvID, &cv); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cv not found"})
		return
	}

	c.JSON(http.StatusOK, cv)
}
