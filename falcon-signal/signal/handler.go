package signal

import (
	"context"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// Module wires the signal pipeline into falcon-signal.
type Module struct{}

func NewModule() *Module { return &Module{} }

func (m *Module) Register(ctx context.Context) error {
	svc, err := newService()
	if err != nil {
		return err
	}

	if err := system.Subscribe(
		ctx,
		constants.StreamMatches,
		"falcon-signal-match-result",
		constants.SubjectMatchResult,
		svc.handleMatchResult,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectMatchResult)

	if err := system.Subscribe(
		ctx,
		constants.StreamSignal,
		"falcon-signal-device-token",
		constants.SubjectSignalDeviceTokenRegister,
		svc.handleRegisterIOSDeviceToken,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectSignalDeviceTokenRegister)

	if err := system.Subscribe(
		ctx,
		constants.StreamSignal,
		"falcon-signal-device-token-logout",
		constants.SubjectSignalDeviceTokenLogout,
		svc.handleLogoutIOSDeviceToken,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectSignalDeviceTokenLogout)

	if err := system.Subscribe(
		ctx,
		constants.StreamSignal,
		"falcon-signal-magic-link",
		constants.SubjectSignalMagicLink,
		svc.handleMagicLink,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectSignalMagicLink)

	if err := system.Subscribe(
		ctx,
		constants.StreamSignal,
		"falcon-signal-admin-alert",
		constants.SubjectSignalAdminAlert,
		svc.handleAdminAlert,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectSignalAdminAlert)

	if err := system.Subscribe(
		ctx,
		constants.StreamSignal,
		"falcon-signal-admin-test-match",
		constants.SubjectSignalAdminTestMatch,
		svc.handleAdminTestMatch,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectSignalAdminTestMatch)

	if err := system.Subscribe(
		ctx,
		constants.StreamSignal,
		"falcon-signal-live-activity-update",
		constants.SubjectSignalLiveActivityUpdate,
		svc.handleLiveActivityUpdateToken,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectSignalLiveActivityUpdate)

	// Start the background flush loop that delivers buffered admin alerts.
	// Events arriving via handleAdminAlert are deduped in the buffer; the
	// loop flushes every ADMIN_ALERT_WINDOW (default 2m) and sends the
	// consolidated notifications via the AdminNotifier.
	go runAlertFlushLoop(ctx, svc.alertBuf, svc.admin)

	return nil
}
