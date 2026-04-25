package main

import (
	_ "embed"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/normalizer/cv"
	"hblabs.co/falcon/normalizer/project"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/llm"
	"hblabs.co/falcon/packages/system"
)

//go:embed prompt.md
var normalizePrompt string

//go:embed prompt_translate.md
var translatePrompt string

//go:embed prompt_cv.md
var cvNormalizePrompt string

func main() {
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.PrintStartupBanner(constants.ServiceNormalizer)

	system.InitBus(system.MergeBusConfigs(system.StreamProjects(), system.StreamMatches(), system.StreamStorage()))

	ctx := system.Ctx()
	if err := system.InitStorage(ctx); err != nil {
		logrus.Fatalf("storage init: %v", err)
	}

	system.RegisterServiceFromBuildTime(ctx, constants.ServiceNormalizer)

	// Shared LLM client — translate prompt is the same for all modules.
	llmClient, err := llm.NewFromEnv(translatePrompt)
	if err != nil {
		logrus.Fatalf("llm client: %v", err)
	}

	// Project normalizer module.
	projectSvc, err := project.NewService(ctx, llmClient, normalizePrompt)
	if err != nil {
		logrus.Fatalf("project service: %v", err)
	}
	if err := projectSvc.Register(ctx); err != nil {
		logrus.Fatalf("project register: %v", err)
	}

	// CV normalizer module.
	cvSvc := cv.NewService(llmClient, cvNormalizePrompt)
	if err := cvSvc.Register(ctx); err != nil {
		logrus.Fatalf("cv register: %v", err)
	}

	logrus.Info("falcon-normalizer started — project + cv modules registered")
	system.Wait()
}
