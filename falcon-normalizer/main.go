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
	ctx := system.Boot(constants.ServiceNormalizer)

	system.InitBus(system.MergeBusConfigs(
		system.StreamProjects(),
		system.StreamMatches(),
		system.StreamStorage(),
	))

	// Shared LLM client — translate prompt is the same for all modules.
	llmClient, err := llm.NewFromEnv(translatePrompt)
	if err != nil {
		logrus.Fatalf("llm client: %v", err)
	}

	projectSvc, err := project.NewService(ctx, llmClient, normalizePrompt)
	if err != nil {
		logrus.Fatalf("project service: %v", err)
	}
	cvSvc := cv.NewService(llmClient, cvNormalizePrompt)

	// project.Service and cv.Service already implement system.Module via
	// their Register(ctx) method — pass them straight in.
	if err := system.RunForever(ctx, projectSvc, cvSvc); err != nil {
		logrus.Fatalf("run: %v", err)
	}
}
