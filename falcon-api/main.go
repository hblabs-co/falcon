package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/api/admin"
	"hblabs.co/falcon/api/auth"
	"hblabs.co/falcon/api/cv"
	"hblabs.co/falcon/api/matches"
	"hblabs.co/falcon/api/me"
	"hblabs.co/falcon/api/projects"
	"hblabs.co/falcon/api/scrape"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/api/signal"
	"hblabs.co/falcon/common/system"
)

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	ctx := system.Ctx()
	if err := system.InitStorage(ctx); err != nil {
		logrus.Fatalf("storage init: %v", err)
	}

	system.InitBus(system.MergeBusConfigs(
		system.StreamScrape(),
		system.StreamStorage(),
		system.StreamSignal(),
	))

	if err := server.Run(
		admin.Routes{},
		auth.Routes{},
		scrape.Routes{},
		cv.Routes{},
		signal.Routes{},
		projects.Routes{},
		matches.Routes{},
		me.Routes{},
	); err != nil {
		logrus.Fatalf("server: %v", err)
	}
}
