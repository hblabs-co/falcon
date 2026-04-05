package scrape

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Service publishes scrape.requested.{platform} events to NATS.
type Service struct{}

func NewService() *Service { return &Service{} }

// Run starts the HTTP server. It blocks until the server stops.
func (s *Service) Run() error {
	port := helpers.ReadEnvOptional("PORT", "8082")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger())
	r.SetTrustedProxies(nil)

	routes(r, s)
	return r.Run(":" + port)
}

// Publish sends a ScrapeRequestedEvent to the platform-specific NATS subject.
func (s *Service) Publish(ctx context.Context, event models.ScrapeRequestedEvent) error {
	subject := fmt.Sprintf("%s.%s", constants.SubjectScrapeRequested, event.Platform)
	return system.Publish(ctx, subject, event)
}
