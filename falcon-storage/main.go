package main

import (
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/storage/company_logo"
	"hblabs.co/falcon/storage/cv"
)

func main() {
	ctx := system.Boot(constants.ServiceStorage)
	// cv.prepare.requested / cv.prepared are NATS core request/reply — not in any stream.
	system.InitBus(system.StreamStorage())

	if err := system.RunForever(ctx, company_logo.NewModule(), cv.NewModule()); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
