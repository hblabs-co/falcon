package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/match-engine/match"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(system.StreamMatches())

	ctx := system.Ctx()
	if err := system.InitStorage(ctx); err != nil {
		logrus.Fatalf("storage init: %v", err)
	}

	svc, err := match.NewService(ctx)
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := svc.Run(); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
