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

// RecordError persists a per-item ServiceError to the "errors" collection and
// returns the ID of the inserted document. Each call creates a fresh document
// with a new nanoid — useful for retry workers and per-job traceability.
//
// Use this when the failure is local to a specific item (a single job's
// inspect 5xx-ed; one CV failed to parse). Use RecordCategoricalError instead
// when the failure is system-wide and should be aggregated into a single
// document with an occurrence counter.
//
// OccurredAt, StackTrace, and a sensible default Priority are auto-filled.
// Returns "" if persistence failed.
func RecordError(ctx context.Context, doc models.ServiceError) string {
	prepareError(&doc)
	log := errorLogger(&doc)

	doc.ID = gonanoid.Must()
	if err := GetStorage().Insert(ctx, constants.MongoErrorsCollection, doc); err != nil {
		log.Errorf("failed to persist service error: %v — original: %s", err, doc.Error)
		return ""
	}
	log.Warnf("service error recorded: %s", doc.Error)
	return doc.ID
}

// RecordCategoricalError persists a system-level ServiceError as a single
// deduped document keyed by a deterministic ID (service:platform:error_name).
// Subsequent occurrences of the same category increment OccurrenceCount and
// refresh the latest message/HTML/priority instead of creating new records.
// Returns the deterministic ID (or "" on failure).
//
// On the FIRST occurrence:
//   - Creates the document with OccurredAt = LastSeenAt = now
//   - OccurrenceCount = 1
//   - Candidate (if provided) is preserved as a permanent "first observed
//     example" via $setOnInsert and is never overwritten on later occurrences.
//
// On SUBSEQUENT occurrences:
//   - LastSeenAt = now
//   - OccurrenceCount += 1
//   - Error / HTML / StackTrace / Priority refreshed to the latest snapshot
//   - Resolved reset to false (re-opened) so the incident is visible again
//   - OccurredAt and Candidate are preserved (they represent when this
//     category first started and which item first triggered it)
func RecordCategoricalError(ctx context.Context, doc models.ServiceError) string {
	prepareError(&doc)
	log := errorLogger(&doc)

	deterministicID := fmt.Sprintf("%s:%s:%s", doc.ServiceName, doc.Platform, doc.ErrorName)
	now := doc.OccurredAt

	setOnInsert := bson.M{
		"id":           deterministicID,
		"service_name": doc.ServiceName,
		"error_name":   doc.ErrorName,
		"platform":     doc.Platform,
		"occurred_at":  now,
		"retry_count":  0,
	}
	if doc.Candidate != nil {
		setOnInsert["candidate"] = doc.Candidate
	}

	filter := bson.M{"id": deterministicID}
	update := bson.M{
		"$setOnInsert": setOnInsert,
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

// prepareError fills the auto-managed fields (StackTrace, OccurredAt, default
// Priority) on a ServiceError. Shared by both record paths so callers stay
// minimal.
func prepareError(doc *models.ServiceError) {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	doc.StackTrace = string(buf[:n])
	doc.OccurredAt = time.Now()
	if doc.Priority == "" {
		doc.Priority = models.ErrorPriorityLow
	}
}

// errorLogger returns a contextual logger pre-tagged with the salient
// identifiers from a ServiceError.
func errorLogger(doc *models.ServiceError) *logrus.Entry {
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
	return log
}
