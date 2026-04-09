package main

import (
	"hblabs.co/falcon/common/system"
	hblabsco "hblabs.co/falcon/scout/platforms/hblabsco"
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

	service := NewService()
	service.
		// RegisterPlatform(actcongmbhde.New()).
		// RegisterPlatform(akkodiscom.New()).
		// RegisterPlatform(computerfuturescom.New()).
		// RegisterPlatform(contractorde.New()).
		// RegisterPlatform(freelancede.New()).
		// RegisterPlatform(gecogroupcom.New()).
		// RegisterPlatform(haysde.New()).
		RegisterPlatform(hblabsco.New()).
		// RegisterPlatform(itcagcom.New()).
		// RegisterPlatform(joyitde.New()).
		// RegisterPlatform(mamgruppecom.New()).
		// RegisterPlatform(peakonede.New()).
		// RegisterPlatform(solcomde.New()).
		// RegisterPlatform(waynicede.New()).
		// RegisterPlatform(wematchde.New()).
		// RegisterPlatform(redglobalde.New()).
		Run()
}
