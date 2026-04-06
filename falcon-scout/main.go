package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/system"
	freelancede "hblabs.co/falcon/scout/platforms/freelance.de"
)

func main() {

	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()
	system.InitBus(system.MergeBusConfigs(
		system.StreamProjects(),
		system.StreamScrape(),
		system.StreamStorage(),
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
