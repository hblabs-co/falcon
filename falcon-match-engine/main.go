package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/match-engine/match"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.PrintStartupBanner(constants.ServiceMatchEngine)

	system.InitBus(system.StreamMatches())

	ctx := system.Ctx()
	if err := system.InitStorage(ctx); err != nil {
		logrus.Fatalf("storage init: %v", err)
	}

	system.RegisterServiceFromBuildTime(ctx, constants.ServiceMatchEngine)

	svc, err := match.NewService(ctx)
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := svc.Run(); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
