package users

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

// downloadUserCV asks falcon-storage (over NATS core request/reply)
// for a presigned MinIO GET URL and 302-redirects the browser to it.
//
// The MinIO client lives on falcon-storage — it owns object-store
// access. Other services (this admin proxy, falcon-api) ask via
// NATS instead of opening their own MinIO connections, mirroring
// the existing upload pattern (cv.prepare.requested) so the data
// flow stays a thin pipeline through the stack.
func downloadUserCV(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}

	req := models.CVDownloadRequestedEvent{
		RequestID: gonanoid.Must(),
		UserID:    id,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	var result models.CVDownloadPreparedEvent
	if err := system.Request(ctx, constants.SubjectCVDownloadRequested, req, &result); err != nil {
		logrus.Errorf("[admin] cv download request: %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "storage service unavailable"})
		return
	}
	if result.NotFound || result.URL == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "no CV on file for this user"})
		return
	}

	logrus.Infof("[admin] presigned CV download for user=%s", id)
	c.Redirect(http.StatusFound, result.URL)
}
