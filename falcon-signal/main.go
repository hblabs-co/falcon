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

	system.InitBus(system.NewBusConfig(
		constants.StreamMatches,
		constants.SubjectMatchPending,
		constants.SubjectMatchResult,
	))

	svc, err := signal.NewService()
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := svc.Run(); err != nil {
		logrus.Fatalf("server: %v", err)
	}
}
