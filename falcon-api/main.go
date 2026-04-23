package main

import (
	"github.com/sirupsen/logrus"

	"hblabs.co/falcon/api/admin"
	"hblabs.co/falcon/api/auth"
	"hblabs.co/falcon/api/companies"
	"hblabs.co/falcon/api/cv"
	"hblabs.co/falcon/api/matches"
	"hblabs.co/falcon/api/me"
	"hblabs.co/falcon/api/projects"
	"hblabs.co/falcon/api/scrape"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/api/signal"
	apisystem "hblabs.co/falcon/api/system"
	"hblabs.co/falcon/common/constants"
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

	// Self-register in the `system` collection on boot so GET /system
	// always reflects the currently-running set of services. Reads
	// BUILD_TIME for the publish date; falls back to now if unset.
	system.RegisterServiceFromBuildTime(ctx, constants.ServiceAPI)

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
		companies.Routes{},
		apisystem.Routes{},
	); err != nil {
		logrus.Fatalf("server: %v", err)
	}
}
