package main

import (
	"hblabs.co/falcon/common/system"
	actcongmbhde "hblabs.co/falcon/scout/platforms/actcon-gmbh.de"
	akkodiscom "hblabs.co/falcon/scout/platforms/akkodis.com"
	computerfuturescom "hblabs.co/falcon/scout/platforms/computerfutures.com"
	contractorde "hblabs.co/falcon/scout/platforms/contractor.de"
	freelancede "hblabs.co/falcon/scout/platforms/freelance.de"
	gecogroupcom "hblabs.co/falcon/scout/platforms/geco-group.com"
	haysde "hblabs.co/falcon/scout/platforms/hays.de"
	hblabsco "hblabs.co/falcon/scout/platforms/hblabs.co"
	itcagcom "hblabs.co/falcon/scout/platforms/itcag.com"
	joyitde "hblabs.co/falcon/scout/platforms/joyit.de"
	mamgruppecom "hblabs.co/falcon/scout/platforms/mamgruppe.com"
	peakonede "hblabs.co/falcon/scout/platforms/peak-one.de"
	redglobalde "hblabs.co/falcon/scout/platforms/redglobal.de"
	solcomde "hblabs.co/falcon/scout/platforms/solcom.de"
	waynicede "hblabs.co/falcon/scout/platforms/waynice.de"
	wematchde "hblabs.co/falcon/scout/platforms/wematch.de"
	"hblabs.co/falcon/scout/scout"
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

	service := scout.NewService()
	service.
		RegisterPlatform(actcongmbhde.New()).
		RegisterPlatform(akkodiscom.New()).
		RegisterPlatform(computerfuturescom.New()).
		RegisterPlatform(contractorde.New()).
		RegisterPlatform(freelancede.New()).
		RegisterPlatform(gecogroupcom.New()).
		RegisterPlatform(haysde.New()).
		RegisterPlatform(hblabsco.New()).
		RegisterPlatform(itcagcom.New()).
		RegisterPlatform(joyitde.New()).
		RegisterPlatform(mamgruppecom.New()).
		RegisterPlatform(peakonede.New()).
		RegisterPlatform(solcomde.New()).
		RegisterPlatform(waynicede.New()).
		RegisterPlatform(wematchde.New()).
		RegisterPlatform(redglobalde.New()).
		Run()
}
