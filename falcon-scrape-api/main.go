package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/scrape-api/scrape"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(system.NewBusConfig(
		constants.StreamScrape,
		constants.SubjectScrapeRequested+".>",
		constants.SubjectScrapeFailed,
	))

	svc := scrape.NewService()

	if err := svc.Run(); err != nil {
		logrus.Fatalf("server: %v", err)
	}

	system.Wait()
	logrus.Info("service stopped")
}
