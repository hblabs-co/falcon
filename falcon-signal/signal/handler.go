package signal

import (
	"context"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
)

var indexes = []system.StorageIndexSpec{
	system.NewIndexSpec(constants.MongoIOSDeviceTokensCollection, "device_id", true),
	system.NewIndexSpec(constants.MongoIOSDeviceTokensCollection, "user_id", false),
}

// Module wires the signal pipeline into falcon-signal.
type Module struct{}

func NewModule() *Module { return &Module{} }

func (m *Module) Register(ctx context.Context) error {
	svc, err := newService()
	if err != nil {
		return err
	}

	if err := system.GetStorage().EnsureIndex(ctx, indexes...); err != nil {
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

	return nil
}
