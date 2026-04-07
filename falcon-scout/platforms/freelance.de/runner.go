package freelancede

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/interfaces"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

var scanRunning atomic.Bool

func getLogger() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{"source": Source})
}

var indexes = []system.StorageIndexSpec{
	system.NewIndexSpec(constants.MongoProjectsCollection, "platform_id", true),
	system.NewIndexSpec(constants.MongoErrorsCollection, "service_name", false),
	system.NewIndexSpec(constants.MongoErrorsCollection, "platform_id", false),
}

func Run() {
	getLogger().Infof("starting — polling every %s (Ctrl+C to stop)", system.PollInterval())

	for _, spec := range indexes {
		if err := system.GetStorage().EnsureIndex(system.Ctx(), spec); err != nil {
			getLogger().Errorf("ensure index %s.%s: %v", spec.Collection, spec.Field, err)
		}
	}
	if err := getSession().Login(); err != nil {
		getLogger().Fatalf("login failed: %v", err)
	}

	StartRetryWorker(system.Ctx())

	system.Poll(system.Ctx(), system.PollInterval(), getLogger(), func() {
		toFetch, err := collectNewCandidates(system.Ctx())
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
		system.BatchProcess(system.Ctx(), toFetch, system.BatchCfg(), processOneCandidate)
	})
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
		return
	}

	saveProject(ctx, c, result, log)
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
