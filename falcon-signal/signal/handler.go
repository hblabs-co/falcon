package signal

import (
	"context"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
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
		svc.handleRegisterToken,
	); err != nil {
		return err
	}
	logrus.Infof("[signal] subscribed → %s", constants.SubjectSignalDeviceTokenRegister)

	return nil
}
