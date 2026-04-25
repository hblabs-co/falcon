package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/scout/platforms/computerfuturescom"
	"hblabs.co/falcon/scout/platforms/constaffcom"
	"hblabs.co/falcon/scout/platforms/contractorde"
	"hblabs.co/falcon/scout/platforms/hblabsco"
	"hblabs.co/falcon/scout/platforms/redglobalde"
	"hblabs.co/falcon/scout/platforms/solcomde"
	"hblabs.co/falcon/scout/platforms/somide"
)

func main() {
	ctx := system.Boot(constants.ServiceScout)

	system.InitBus(system.MergeBusConfigs(
		system.StreamProjects(),
		system.StreamScrape(),
		system.StreamStorage(),
		system.StreamSignal(),
	))

	service := NewService().
		// RegisterPlatform(actcongmbhde.New()).
		// RegisterPlatform(akkodiscom.New()).
		RegisterPlatform(computerfuturescom.New()).
		RegisterPlatform(contractorde.New()).
		RegisterPlatform(constaffcom.New()).
		// RegisterPlatform(freelancede.New()).
		// RegisterPlatform(gecogroupcom.New()).
		// RegisterPlatform(haysde.New()).
		RegisterPlatform(hblabsco.New()).
		// RegisterPlatform(itcagcom.New()).
		// RegisterPlatform(joyitde.New()).
		// RegisterPlatform(mamgruppecom.New()).
		// RegisterPlatform(peakonede.New()).
		RegisterPlatform(solcomde.New()).
		RegisterPlatform(somide.New()).
		// RegisterPlatform(waynicede.New()).
		// RegisterPlatform(wematchde.New()).
		RegisterPlatform(redglobalde.New())

	if err := system.RunForever(ctx, service); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
