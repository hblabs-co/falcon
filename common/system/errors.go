package system

import (
	"context"
	"fmt"
	"runtime"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
)

// RecordError persists a ServiceError to the "errors" collection and returns
// the ID of the resulting document so callers can reference it (e.g. when
// publishing an admin alert event that points back at the record).
//
// It distinguishes between two kinds of errors based on whether doc.Candidate
// is nil:
//
//   - Per-item error (Candidate != nil): inserted as a brand-new document with
//     a fresh nanoid. Multiple occurrences of the same error against different
//     items produce separate records — useful for retry workers and traceability.
//
//   - Categorical error (Candidate == nil): represents a system-level anomaly
//     that affects ALL items (markup drift, repeated infrastructure failure).
//     These are deduped via a deterministic ID (service:platform:error_name)
//     and upserted: subsequent occurrences update LastSeenAt, refresh the latest
//     Error/HTML/Priority, and atomically increment OccurrenceCount. This avoids
//     flooding the errors collection with one record per poll cycle.
//
// In both cases OccurredAt, StackTrace, and a sensible default Priority are
// auto-filled so callers only need to set the domain fields. Returns "" if
// persistence failed.
func RecordError(ctx context.Context, doc models.ServiceError) string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
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

	if doc.Candidate == nil {
		return recordCategoricalError(ctx, doc, log)
	}

	doc.ID = gonanoid.Must()
	if err := GetStorage().Insert(ctx, constants.MongoErrorsCollection, doc); err != nil {
		log.Errorf("failed to persist service error: %v — original: %s", err, doc.Error)
		return ""
	}
	log.Warnf("service error recorded: %s", doc.Error)
	return doc.ID
}

// recordCategoricalError upserts a system-level (non-per-item) error keyed by a
// deterministic ID so that recurrent failures only produce ONE document with a
// counter, instead of one document per poll cycle. Returns the deterministic ID
// (or "" on failure) so callers can reference it from downstream events.
//
// On first occurrence:
//   - Creates the document with OccurredAt = LastSeenAt = now
//   - OccurrenceCount = 1
//
// On subsequent occurrences:
//   - LastSeenAt = now
//   - OccurrenceCount += 1
//   - Error / HTML / Priority refreshed to the latest
//   - Resolved reset to false (re-opened) so the incident is visible again
//   - OccurredAt is preserved (it represents when this category first started)
func recordCategoricalError(ctx context.Context, doc models.ServiceError, log *logrus.Entry) string {
	deterministicID := fmt.Sprintf("%s:%s:%s", doc.ServiceName, doc.Platform, doc.ErrorName)
	now := doc.OccurredAt

	filter := bson.M{"id": deterministicID}
	update := bson.M{
		"$setOnInsert": bson.M{
			"id":           deterministicID,
			"service_name": doc.ServiceName,
			"error_name":   doc.ErrorName,
			"platform":     doc.Platform,
			"occurred_at":  now,
			"retry_count":  0,
		},
		"$set": bson.M{
			"error":        doc.Error,
			"stack_trace":  doc.StackTrace,
			"priority":     doc.Priority,
			"html":         doc.HTML,
			"last_seen_at": now,
			"resolved":     false,
		},
		"$inc": bson.M{
			"occurrence_count": 1,
		},
	}

	if err := GetStorage().RawUpdate(ctx, constants.MongoErrorsCollection, filter, update); err != nil {
		log.Errorf("failed to upsert categorical error: %v — original: %s", err, doc.Error)
		return ""
	}
	log.Warnf("categorical error recorded: %s", doc.Error)
	return deterministicID
}
