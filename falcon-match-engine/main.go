package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/match-engine/match"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

func main() {
	ctx := system.Boot(constants.ServiceMatchEngine)
	system.InitBus(system.StreamMatches())

	svc, err := match.NewService(ctx)
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := system.RunForever(ctx, svc); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
