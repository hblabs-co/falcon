package system

import (
	"context"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
)

// RecordWarning persists a ServiceWarning to the "warnings" collection and
// returns the ID of the inserted document so callers can reference it (e.g.
// when publishing an admin alert event that points back at the record).
//
// It automatically fills ID, OccurredAt, and Priority so callers only need to
// set the domain fields (ServiceName, WarningName, Message, Platform, Candidate).
// Returns "" if persistence failed.
func RecordWarning(ctx context.Context, doc models.ServiceWarning) string {
	doc.ID = gonanoid.Must()
	doc.OccurredAt = time.Now()
	if doc.Priority == "" {
		doc.Priority = models.WarningPriorityLow
	}

	log := logrus.WithFields(logrus.Fields{
		"service":      doc.ServiceName,
		"warning_name": doc.WarningName,
	})
	if doc.Platform != "" {
		log = log.WithField("platform", doc.Platform)
	}

	if err := GetStorage().Insert(ctx, constants.MongoWarningsCollection, doc); err != nil {
		log.Errorf("failed to persist service warning: %v — original: %s", err, doc.Message)
		return ""
	}
	log.Warnf("service warning recorded: %s", doc.Message)
	return doc.ID
}
