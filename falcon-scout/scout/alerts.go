package main

import (
	"context"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// publishAdminAlert publishes an AdminAlertEvent to signal.admin_alert so
// falcon-signal can fan it out to the operations team. The event is tiny
// (kind + id) — signal loads the full record from MongoDB.
func publishAdminAlert(ctx context.Context, kind models.AdminAlertKind, id string) {
	evt := models.AdminAlertEvent{Kind: kind, ID: id}
	if err := system.Publish(ctx, constants.SubjectSignalAdminAlert, evt); err != nil {
		logrus.Errorf("publish admin alert (%s %s): %v", kind, id, err)
	}
}
