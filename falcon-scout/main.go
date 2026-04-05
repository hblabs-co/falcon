package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
	freelancede "hblabs.co/falcon/scout/platforms/freelance.de"
)

func main() {

	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()
	system.InitBus(append(
		system.NewBusConfig(
			constants.StreamProjects,
			constants.SubjectProjectCreated,
			constants.SubjectProjectUpdated,
		),
		system.NewBusConfig(
			constants.StreamScrape,
			constants.SubjectScrapeRequested+".>",
			constants.SubjectScrapeFailed,
		)...,
	))

	RunScrapeConsumer()

	go freelancede.Run()

	// go protalx.Run()
	// go protaly.Run()
	// go protalz.Run()
	// go freelancemap.Run()

	system.Wait()
	logrus.Info("all scout platforms stopped")
}
