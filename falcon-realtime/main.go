package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/realtime/realtime"
)

func main() {
	ctx := system.Boot(constants.ServiceRealtime, system.WithPort(realtime.GetParsedPort()))

	system.InitBus(system.MergeBusConfigs(
		system.StreamRealtime(),
		system.StreamProjects(),
		system.StreamMatches(),
	))

	if err := system.RunForever(ctx, realtime.NewModule()); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
