package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/storage/company_logo"
	"hblabs.co/falcon/storage/cv"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.PrintStartupBanner(constants.ServiceStorage)
	system.RegisterServiceFromBuildTime(system.Ctx(), constants.ServiceStorage)

	// cv.prepare.requested / cv.prepared are NATS core request/reply — not in any stream.
	system.InitBus(system.StreamStorage())

	if err := system.Run(
		system.Ctx(),
		company_logo.NewModule(),
		cv.NewModule(),
	); err != nil {
		logrus.Fatalf("start: %v", err)
	}

	system.Wait()
	logrus.Info("service stopped")
}
