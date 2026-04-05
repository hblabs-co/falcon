package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/dispatch/dispatch"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(system.MergeBusConfigs(system.NewBusConfig(
		constants.StreamProjects,
		constants.SubjectProjectCreated,
		constants.SubjectProjectUpdated,
	)))

	svc, err := dispatch.NewService()
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := svc.Run(); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
