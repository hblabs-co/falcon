package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/signal/signal"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(system.MergeBusConfigs(
		system.NewBusConfig(
			constants.StreamSignal,
			constants.SubjectSignalDeviceTokenRegister,
		),
		system.NewBusConfig(
			constants.StreamMatches,
			constants.SubjectMatchPending,
			constants.SubjectMatchResult,
		)))

	if err := system.Run(system.Ctx(), signal.NewModule()); err != nil {
		logrus.Fatalf("start: %v", err)
	}

	system.Wait()
	logrus.Info("service stopped")
}
