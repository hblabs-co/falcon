package system

import (
	"context"
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
)

// RecordWarning persists a per-item ServiceWarning to the "warnings"
// collection and returns the ID of the inserted document. Each call creates a
// fresh document with a new nanoid — useful when the warning is about a
// specific record (one job's data, one CV) and per-event history matters.
//
// Use RecordCategoricalWarning instead for system-wide anomalies that should
// be aggregated into a single document with an occurrence counter.
//
// Returns "" if persistence failed.
func RecordWarning(ctx context.Context, doc models.ServiceWarning) string {
	prepareWarning(&doc)
	log := warningLogger(&doc)

	doc.ID = gonanoid.Must()
	if err := GetStorage().Insert(ctx, constants.MongoWarningsCollection, doc); err != nil {
		log.Errorf("failed to persist service warning: %v — original: %s", err, doc.Message)
		return ""
	}
	log.Warnf("service warning recorded: %s", doc.Message)
	return doc.ID
}

// RecordCategoricalWarning persists a system-level ServiceWarning as a single
// deduped document keyed by a deterministic ID (service:platform:warning_name).
// Subsequent occurrences of the same category increment OccurrenceCount and
// refresh the latest message/HTML/priority instead of creating new records.
// Returns the deterministic ID (or "" on failure).
//
// Same semantics as RecordCategoricalError: Candidate (if provided) is
// preserved as a permanent "first observed example" via $setOnInsert.
func RecordCategoricalWarning(ctx context.Context, doc models.ServiceWarning) string {
	prepareWarning(&doc)
	log := warningLogger(&doc)

	deterministicID := fmt.Sprintf("%s:%s:%s", doc.ServiceName, doc.Platform, doc.WarningName)
	now := doc.OccurredAt

	setOnInsert := bson.M{
		"id":           deterministicID,
		"service_name": doc.ServiceName,
		"warning_name": doc.WarningName,
		"platform":     doc.Platform,
		"occurred_at":  now,
	}
	if doc.Candidate != nil {
		setOnInsert["candidate"] = doc.Candidate
	}

	filter := bson.M{"id": deterministicID}
	update := bson.M{
		"$setOnInsert": setOnInsert,
		"$set": bson.M{
			"message":      doc.Message,
			"priority":     doc.Priority,
			"html":         doc.HTML,
			"last_seen_at": now,
			"resolved":     false,
		},
		"$inc": bson.M{
			"occurrence_count": 1,
		},
	}

	if err := GetStorage().RawUpdate(ctx, constants.MongoWarningsCollection, filter, update); err != nil {
		log.Errorf("failed to upsert categorical warning: %v — original: %s", err, doc.Message)
		return ""
	}
	log.Warnf("categorical warning recorded: %s", doc.Message)
	return deterministicID
}

// prepareWarning fills auto-managed fields on a ServiceWarning. Shared by
// both record paths.
func prepareWarning(doc *models.ServiceWarning) {
	doc.OccurredAt = time.Now()
	if doc.Priority == "" {
		doc.Priority = models.WarningPriorityLow
	}
}

// warningLogger returns a contextual logger pre-tagged with the salient
// identifiers from a ServiceWarning.
func warningLogger(doc *models.ServiceWarning) *logrus.Entry {
	log := logrus.WithFields(logrus.Fields{
		"service":      doc.ServiceName,
		"warning_name": doc.WarningName,
	})
	if doc.Platform != "" {
		log = log.WithField("platform", doc.Platform)
	}
	return log
}
