package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/dispatch/dispatch"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

func main() {
	ctx := system.Boot(constants.ServiceDispatch)
	system.InitBus(system.StreamProjects())

	svc, err := dispatch.NewService()
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := system.RunForever(ctx, svc); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
