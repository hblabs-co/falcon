package issues

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// resolveOne flips one row's `resolved` to true, scoped to the
// right collection by the :type path param. The id is a 1:1
// match (no aggregation, no fanout).
func resolveOne(c *gin.Context) {
	typ := c.Param("type")
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	collection, ok := collectionFor(typ)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'error' or 'warning'"})
		return
	}
	if err := system.GetStorage().RawUpdate(c.Request.Context(), collection,
		bson.M{"id": id},
		bson.M{"$set": bson.M{"resolved": true, "resolved_at": time.Now()}},
	); err != nil {
		respondInternal(c, "resolve "+typ+" "+id, err)
		return
	}
	logrus.Infof("[admin] resolved %s id=%s", typ, id)
	c.Status(http.StatusNoContent)
}

// resolveAll bulk-resolves every row matching a filter, scoped by
// type (defaults to "all" which fans out across both
// collections). Useful for clearing a noisy day in one click;
// optional `service` query narrows the blast radius.
func resolveAll(c *gin.Context) {
	typ := c.DefaultQuery("type", "all")
	service := c.Query("service")

	filter := bson.M{"resolved": bson.M{"$ne": true}}
	if service != "" {
		filter["service_name"] = service
	}
	patch := bson.M{"$set": bson.M{"resolved": true, "resolved_at": time.Now()}}

	ctx := c.Request.Context()
	if typ == TypeError || typ == "all" {
		if _, err := system.GetStorage().BulkUpdate(ctx,
			constants.MongoErrorsCollection, filter, patch); err != nil {
			respondInternal(c, "bulk resolve errors", err)
			return
		}
	}
	if typ == TypeWarning || typ == "all" {
		if _, err := system.GetStorage().BulkUpdate(ctx,
			constants.MongoWarningsCollection, filter, patch); err != nil {
			respondInternal(c, "bulk resolve warnings", err)
			return
		}
	}
	logrus.Infof("[admin] bulk-resolved type=%s service=%s", typ, service)
	c.Status(http.StatusNoContent)
}

func collectionFor(typ string) (string, bool) {
	switch typ {
	case TypeError:
		return constants.MongoErrorsCollection, true
	case TypeWarning:
		return constants.MongoWarningsCollection, true
	}
	return "", false
}
