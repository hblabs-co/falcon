package freelancede

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/interfaces"
)

var scanRunning atomic.Bool

func getLogger() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{"source": Source})
}

// Runner implements the Platform interface for freelance.de.
type Runner struct {
	logger interfaces.Logger
}

func New() *Runner { return &Runner{} }

func (r *Runner) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

func (r *Runner) Name() string {
	return Source
}

func (r *Runner) Init(ctx context.Context) error {

	if err := getSession().Login(); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	return nil
}

func (r *Runner) StartConsumers(ctx context.Context) error {
	return registerConsumers(ctx)
}

func (r *Runner) StartWorkers(ctx context.Context) {
	StartRetryWorker(ctx)
}

func (r *Runner) Poll(ctx context.Context) {
	system.Poll(ctx, system.PollInterval(), getLogger(), func() {
		toFetch, err := collectNewCandidates(ctx)
		if err != nil {
			getLogger().Errorf("collect candidates: %v", err)
			return
		}

		total := len(toFetch)
		if total == 0 {
			getLogger().Info("no new or updated projects")
			return
		}

		helpers.Reverse(&toFetch)
		for i, c := range toFetch {
			c.Total = total
			c.Current = i + 1
		}

		getLogger().Infof("%d projects to fetch", total)
		system.BatchProcess(ctx, toFetch, system.BatchCfg(), processOneCandidate)
	})
}

func registerConsumers(ctx context.Context) error {
	platform := system.Platform()
	subject := fmt.Sprintf("%s.%s", constants.SubjectScrapeRequested, platform)
	consumer := fmt.Sprintf("scout-%s", strings.ReplaceAll(platform, ".", "-"))

	if err := system.Subscribe(ctx, constants.StreamScrape, consumer, subject, handleScrapeRequested); err != nil {
		return fmt.Errorf("subscribe %s: %w", subject, err)
	}
	getLogger().Infof("subscribed → %s", subject)

	if err := system.Subscribe(ctx, constants.StreamScrape, "scout-scan-today", constants.SubjectScrapeScanToday, handleScanToday); err != nil {
		return fmt.Errorf("subscribe %s: %w", constants.SubjectScrapeScanToday, err)
	}
	getLogger().Infof("subscribed → %s", constants.SubjectScrapeScanToday)
	return nil
}

func handleScrapeRequested(data []byte) error {
	var event models.ScrapeRequestedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal scrape.requested: %w", err)
	}
	getLogger().Infof("on-demand scrape: url=%s", event.URL)
	ScrapeURL(context.Background(), event.URL)
	return nil
}

func handleScanToday(_ []byte) error {
	getLogger().Info("scan-today triggered")
	ScanToday(context.Background())
	return nil
}

// ScanToday collects and processes all of today's candidates. Triggered via admin endpoint.
// Only one scan can run at a time — concurrent calls are silently ignored.
func ScanToday(ctx context.Context) {
	if !scanRunning.CompareAndSwap(false, true) {
		getLogger().Warn("scan today: already running, skipping")
		return
	}
	defer scanRunning.Store(false)

	toFetch, err := collectTodayCandidates(ctx)
	if err != nil {
		getLogger().Errorf("scan today: %v", err)
		return
	}
	total := len(toFetch)
	if total == 0 {
		getLogger().Info("scan today: no candidates found")
		return
	}
	helpers.Reverse(&toFetch)
	for i, c := range toFetch {
		c.Total = total
		c.Current = i + 1
	}
	getLogger().Infof("scan today: %d projects to fetch", total)
	system.BatchProcess(ctx, toFetch, system.BatchCfg(), processOneCandidate)
}

// ScrapeURL handles an on-demand scrape for a single URL received via NATS.
func ScrapeURL(ctx context.Context, url string) {
	candidate := &ProjectCandidate{
		PlatformID: platformIDRe.FindString(url),
		URL:        url,
		Source:     Source,
		Current:    1,
		Total:      1,
	}
	processOneCandidate(ctx, candidate)
}

// processOneCandidate makes a single inspect attempt.
// On failure it records an error with the full candidate so the retry worker can reprocess it.
func processOneCandidate(ctx context.Context, c *ProjectCandidate) {
	log := getLogger().WithFields(logrus.Fields{
		"platform_id": c.PlatformID,
		"current":     c.Current,
		"total":       c.Total,
		"url":         c.URL,
	})

	inspector := &Inspector{Url: c.URL, PlatformID: c.PlatformID, Current: c.Current, Total: c.Total}
	result, err := inspector.Inspect()
	if err != nil {
		processOneCandidateError(ctx, c, inspector, err, log)
		return
	}

	saveProject(ctx, c, result, log)
}

// processOneCandidateError classifies the inspect error and either discards it or records it for retry.
func processOneCandidateError(ctx context.Context, c *ProjectCandidate, inspector *Inspector, err error, log *logrus.Entry) {
	if errors.Is(err, ErrGone) {
		log.Infof("project gone (410) — skipping")
		return
	}

	errName := constants.ErrNameScrapeInspectFailed
	if IsServerError(err) {
		errName = constants.ErrNameScrapeServerError
	}

	log.Warnf("inspect failed (%s): %v — recording for retry", errName, err)
	system.RecordError(ctx, models.ServiceError{
		ServiceName: constants.ServiceScout,
		ErrorName:   errName,
		Error:       err.Error(),
		Platform:    Source,
		PlatformID:  c.PlatformID,
		URL:         c.URL,
		HTML:        inspector.HTML,
		Candidate:   c,
	})
}

// saveProject persists a successfully inspected project and publishes its event.
func saveProject(ctx context.Context, c *ProjectCandidate, result interfaces.Project, log *logrus.Entry) {
	p := models.NewPersistedProject(result, c.PlatformID, Source, time.Now(), c.ExistingID)
	p.PlatformUpdatedAt = c.PlatformUpdatedAt
	if err := system.GetStorage().Replace(ctx, constants.MongoProjectsCollection, p); err != nil {
		log.Errorf("replace project %s: %v", p.PlatformID, err)
		return
	}
	log.Infof("project saved with internal id %s", p.GetId())

	c.UpdateFromResult(result)
	if c.CompanyID != "" {
		evt := c.GetDownloadCompanyLogoRequestEvent()
		subject := constants.SubjectStorageCompanyLogoRequested
		if err := system.Publish(ctx, subject, evt); err != nil {
			log.Errorf("publish %s for company %s: %v", subject, c.CompanyID, err)
		} else {
			log.Infof("company logo request queued — company_id=%s", c.CompanyID)
		}
	} else {
		log.Warnf("skipping company logo — no company_id on candidate")
	}

	subject := constants.SubjectProjectCreated
	if c.ExistingID != "" {
		subject = constants.SubjectProjectUpdated
	}
	if err := system.Publish(ctx, subject, p.GetEvent()); err != nil {
		log.Errorf("publish %s: %v", subject, err)
	} else {
		log.Infof("published %s for %s", subject, p.GetId())
	}
}
