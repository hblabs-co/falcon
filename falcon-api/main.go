package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/api/cv"
	"hblabs.co/falcon/api/scrape"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/api/signal"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(system.MergeBusConfigs(
		system.NewBusConfig(
			constants.StreamScrape,
			constants.SubjectScrapeRequested+".>",
			constants.SubjectScrapeFailed,
		),
		system.NewBusConfig(
			constants.StreamStorage,
			constants.SubjectCVIndexRequested,
			constants.SubjectCVIndexed,
		),
		system.NewBusConfig(
			constants.StreamSignal,
			constants.SubjectSignalDeviceTokenRegister,
		)))

	if err := server.Run(
		scrape.Routes{},
		cv.Routes{},
		signal.Routes{},
	); err != nil {
		logrus.Fatalf("server: %v", err)
	}
}
