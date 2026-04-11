package system

import (
	"context"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
)

// RecordWarning persists a ServiceWarning to the "warnings" collection.
// It automatically fills ID, OccurredAt, and Priority so callers only need to set
// the domain fields (ServiceName, WarningName, Message, Platform, Candidate).
func RecordWarning(ctx context.Context, doc models.ServiceWarning) {
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
		return
	}
	log.Warnf("service warning recorded: %s", doc.Message)
}
