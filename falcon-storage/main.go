package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/storage/company_logo"
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

	err := system.Run(
		system.Ctx(),
		company_logo.NewModule(),
		// next_module.NewModule(),
	)
	if err != nil {
		logrus.Fatalf("start: %v", err)
	}

	system.Wait()
	logrus.Info("service stopped")
}
