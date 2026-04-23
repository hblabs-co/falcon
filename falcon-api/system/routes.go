package system

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	commonsystem "hblabs.co/falcon/common/system"
)

// Routes exposes the public GET /system endpoint. Public on purpose:
// the response is not personalised and carries only service-level
// metadata (publish dates today, version/git-commit/etc. later) that
// the iOS app polls once a minute. Keeping it auth-free means every
// install can read system state without a session token, which is
// necessary since fresh installs run through first-launch checks
// before any login flow.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.GET("/system", handleListSystem)
}

// serviceEntry is the per-service shape the iOS client decodes. We
// return only the well-known fields today — expanding later means
// adding a field here and a corresponding bson key in the collection
// constants file, with no breakage for older clients (Swift Codable
// ignores unknown fields).
type serviceEntry struct {
	ServiceName string    `json:"service_name"`
	PublishDate time.Time `json:"publish_date"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type listResponse struct {
	Services  []serviceEntry `json:"services"`
	UpdatedAt time.Time      `json:"updated_at"`
	Count     int            `json:"count"`
}

func handleListSystem(c *gin.Context) {
	ctx := c.Request.Context()

	var docs []bson.M
	if err := commonsystem.GetStorage().GetMany(ctx, constants.MongoSystemCollection,
		bson.M{}, &docs); err != nil {
		logrus.Errorf("list system: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list system"})
		return
	}

	entries := make([]serviceEntry, 0, len(docs))
	for _, d := range docs {
		e := serviceEntry{}
		if v, ok := d[constants.SystemFieldServiceName].(string); ok {
			e.ServiceName = v
		}
		if v, ok := d[constants.SystemFieldPublishDate].(bson.DateTime); ok {
			e.PublishDate = v.Time()
		}
		if v, ok := d[constants.SystemFieldUpdatedAt].(bson.DateTime); ok {
			e.UpdatedAt = v.Time()
		}
		entries = append(entries, e)
	}

	c.JSON(http.StatusOK, listResponse{
		Services:  entries,
		UpdatedAt: time.Now().UTC(),
		Count:     len(entries),
	})
}
