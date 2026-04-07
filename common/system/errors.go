package system

import (
	"context"
	"runtime"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
)

// RecordError persists a ServiceError to the "errors" collection.
// It automatically fills ID, StackTrace, OccurredAt, and Priority so callers only need to
// set the domain fields (ServiceName, ErrorName, Error, and any optional fields).
func RecordError(ctx context.Context, doc models.ServiceError) {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	doc.ID = gonanoid.Must()
	doc.StackTrace = string(buf[:n])
	doc.OccurredAt = time.Now()
	if doc.Priority == "" {
		doc.Priority = models.ErrorPriorityLow
	}

	log := logrus.WithFields(logrus.Fields{
		"service":    doc.ServiceName,
		"error_name": doc.ErrorName,
	})
	if doc.ProjectID != "" {
		log = log.WithField("project_id", doc.ProjectID)
	}
	if doc.PlatformID != "" {
		log = log.WithField("platform_id", doc.PlatformID)
	}

	if err := GetStorage().Insert(ctx, constants.MongoErrorsCollection, doc); err != nil {
		log.Errorf("failed to persist service error: %v — original: %s", err, doc.Error)
		return
	}
	log.Warnf("service error recorded: %s", doc.Error)
}
