package main

import (
	_ "embed"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/system"

	"hblabs.co/falcon/normalizer/normalizer"
)

//go:embed prompt.md
var systemPrompt string

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.InitBus(system.StreamProjects())

	ctx := system.Ctx()
	if err := system.InitStorage(ctx); err != nil {
		logrus.Fatalf("storage init: %v", err)
	}

	svc, err := normalizer.NewService(ctx, systemPrompt)
	if err != nil {
		logrus.Fatalf("service init: %v", err)
	}

	if err := svc.Run(); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
