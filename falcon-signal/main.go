package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/signal/signal"
)

func main() {
	ctx := system.Boot(constants.ServiceSignal)

	system.InitBus(system.MergeBusConfigs(
		system.StreamSignal(),
		system.StreamMatches(),
	))

	if err := system.RunForever(ctx, signal.NewModule()); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
