package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/realtime/realtime"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.PrintStartupBannerAndPort(constants.ServiceRealtime, realtime.GetParsedPort())
	system.RegisterServiceFromBuildTime(system.Ctx(), constants.ServiceRealtime)

	system.InitBus(system.MergeBusConfigs(
		system.StreamRealtime(),
		system.StreamProjects(),
		system.StreamMatches(),
	))

	if err := system.Run(system.Ctx(), realtime.NewModule()); err != nil {
		logrus.Fatalf("start: %v", err)
	}

	system.Wait()
	logrus.Info("service stopped")
}
