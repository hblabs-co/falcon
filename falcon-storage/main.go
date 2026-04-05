package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/storage/logo"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(system.NewBusConfig(
		constants.StreamStorage,
		constants.SubjectStorageCompanyLogoRequested,
		constants.SubjectStorageCompanyLogoDownloaded,
	))

	svc, err := logo.NewService()
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := svc.Subscribe(); err != nil {
		logrus.Fatalf("subscribe: %v", err)
	}

	system.Wait()
	logrus.Info("service stopped")
}
