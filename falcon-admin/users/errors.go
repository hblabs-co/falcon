package users

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// respondInternal logs the underlying cause server-side and returns
// a generic 500 — clients see "internal error", operators see the
// real message in the log. Used by every handler in this package.
func respondInternal(c *gin.Context, what string, err error) {
	logrus.Errorf("[admin] %s: %v", what, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
