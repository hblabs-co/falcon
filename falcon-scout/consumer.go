package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	freelancede "hblabs.co/falcon/scout/platforms/freelance.de"
)

// RunScrapeConsumer subscribes to scrape.requested.{platform} and processes
// on-demand URLs by delegating to the appropriate platform runner.
// Must be called after InitBus.
func RunScrapeConsumer() {
	platform := system.Platform()
	subject := fmt.Sprintf("%s.%s", constants.SubjectScrapeRequested, platform)
	consumer := fmt.Sprintf("scout-%s", strings.ReplaceAll(platform, ".", "-"))

	if err := system.Subscribe(system.Ctx(), constants.StreamScrape, consumer, subject, handleScrapeRequested); err != nil {
		logrus.Fatalf("subscribe %s: %v", subject, err)
	}
	logrus.Infof("subscribed to %s (consumer: %s)", subject, consumer)

	if err := system.Subscribe(system.Ctx(), constants.StreamScrape, "scout-scan-today", constants.SubjectScrapeScanToday, handleScanToday); err != nil {
		logrus.Fatalf("subscribe %s: %v", constants.SubjectScrapeScanToday, err)
	}
	logrus.Infof("subscribed to %s", constants.SubjectScrapeScanToday)
}

func handleScanToday(data []byte) error {
	logrus.Info("scan-today triggered — collecting all of today's candidates")
	freelancede.ScanToday(context.Background())
	return nil
}

func handleScrapeRequested(data []byte) error {
	var event models.ScrapeRequestedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal scrape.requested: %w", err)
	}
	logrus.Infof("on-demand scrape: platform=%s url=%s", event.Platform, event.URL)
	freelancede.ScrapeURL(context.Background(), event.URL)
	return nil
}
