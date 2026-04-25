package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/signal/signal"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.PrintStartupBanner(constants.ServiceSignal)
	system.RegisterServiceFromBuildTime(system.Ctx(), constants.ServiceSignal)

	system.InitBus(system.MergeBusConfigs(
		system.StreamSignal(),
		system.StreamMatches(),
	))

	if err := system.Run(system.Ctx(), signal.NewModule()); err != nil {
		logrus.Fatalf("start: %v", err)
	}

	system.Wait()
	logrus.Info("service stopped")
}
