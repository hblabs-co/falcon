package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/api/cv"
	"hblabs.co/falcon/api/scrape"
	"hblabs.co/falcon/api/server"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(append(
		system.NewBusConfig(
			constants.StreamScrape,
			constants.SubjectScrapeRequested+".>",
			constants.SubjectScrapeFailed,
		),
		system.NewBusConfig(
			constants.StreamStorage,
			constants.SubjectCVIndexRequested,
			constants.SubjectCVIndexed,
		)...,
	))

	if err := server.Run(
		scrape.Routes{},
		cv.Routes{},
	); err != nil {
		logrus.Fatalf("server: %v", err)
	}
}
