package scout

import (
	"context"
	"strings"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/interfaces"
	"hblabs.co/falcon/common/system"
)

var indexes = []system.StorageIndexSpec{
	system.NewIndexSpec(constants.MongoProjectsCollection, "platform_id", true),
	system.NewIndexSpec(constants.MongoErrorsCollection, "service_name", false),
	system.NewIndexSpec(constants.MongoErrorsCollection, "platform_id", false),
}

// Platform is the contract every scraping platform must fulfill.
type Platform interface {

	// Name returns the platform identifier (e.g. "freelance.de"). Must be unique
	Name() string

	SetLogger(logger interfaces.Logger)

	// Init performs one-time setup: DB indexes, session login, etc.
	Init(ctx context.Context) error

	// Subscribe registers NATS consumers for on-demand scraping and admin triggers.
	StartConsumers(ctx context.Context) error

	// StartWorkers launches background goroutines (retry workers, etc.).
	StartWorkers(ctx context.Context)

	// Poll starts the main polling loop. Blocks until ctx is cancelled.
	Poll(ctx context.Context)
}

// Service orchestrates one or more Platform implementations.
type Service struct {
	platforms           []Platform
	AllowedPlatformsMap map[string]bool
}

func NewService() *Service {
	return &Service{
		platforms:           []Platform{},
		AllowedPlatformsMap: map[string]bool{},
	}
}

func (s *Service) RegisterPlatform(platform Platform) *Service {
	if platform == nil {
		return s
	}

	for _, p := range s.platforms {
		if p.Name() == platform.Name() {
			return s
		}
	}

	s.platforms = append(s.platforms, platform)
	return s
}

func (s *Service) readAllowedPlatforms() {
	envPlatforms := helpers.ReadEnvOptional("PLATFORMS", "hblabs.co")

	parts := strings.Split(envPlatforms, ",")
	s.AllowedPlatformsMap = make(map[string]bool, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		s.AllowedPlatformsMap[p] = true
	}
}

func (s *Service) shouldRun(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	_, ok := s.AllowedPlatformsMap[name]
	return ok
}

func (s *Service) Run() {
	ctx := system.Ctx()
	s.readAllowedPlatforms()

	if err := system.GetStorage().EnsureIndex(ctx, indexes...); err != nil {
		logrus.Fatalf("ensure indexes: %v", err)
	}

	for _, p := range s.platforms {

		if !s.shouldRun(p.Name()) {
			continue
		}

		logger := logrus.WithField("platform", p.Name())
		p.SetLogger(logger)

		if err := p.Init(ctx); err != nil {
			logger.Fatalf("init: %v", err)
		}

		if err := p.StartConsumers(ctx); err != nil {
			logger.Fatalf("subscribe: %v", err)
		}

		go p.StartWorkers(ctx)
		go p.Poll(ctx)

		logger.Info("platform registered and running")
	}

	system.Wait()
	logrus.Info("all scout platforms stopped")
}
