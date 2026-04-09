package redglobalde

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/interfaces"
)

func getLogger() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{"source": Source})
}

// Runner implements the Platform interface for redglobal.de.
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
	return nil
}

func (r *Runner) StartConsumers(ctx context.Context) error {
	return registerConsumers(ctx)
}

func (r *Runner) StartWorkers(ctx context.Context) {
}

func (r *Runner) Poll(ctx context.Context) {
	system.Poll(ctx, system.PollInterval(), r.logger, func() {
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

// collectNewCandidates scrapes listing pages starting from page 1.
// It stops when a page contains candidates that are already persisted.
func collectNewCandidates(ctx context.Context) ([]*ProjectCandidate, error) {
	candidates, err := collectCandidates(1, func(page int, found []*ProjectCandidate) bool {
		existing := lookupExisting(ctx, found)
		return len(existing) == 0
	})
	if err != nil {
		return nil, err
	}
	return filterCandidates(ctx, candidates), nil
}

// lookupExisting returns a map of platformID → PersistedProject for candidates that already exist.
func lookupExisting(ctx context.Context, candidates []*ProjectCandidate) map[string]models.PersistedProject {
	ids := make([]string, len(candidates))
	for i, c := range candidates {
		ids[i] = c.PlatformID
	}
	var existing []models.PersistedProject
	if err := system.GetStorage().GetManyByField(ctx, constants.MongoProjectsCollection, "platform_id", ids, &existing); err != nil {
		getLogger().Errorf("lookup existing: %v", err)
		return nil
	}
	m := make(map[string]models.PersistedProject, len(existing))
	for _, p := range existing {
		m[p.PlatformID] = p
	}
	return m
}

// filterCandidates removes candidates that are already persisted.
func filterCandidates(ctx context.Context, candidates []*ProjectCandidate) []*ProjectCandidate {
	existing := lookupExisting(ctx, candidates)
	var filtered []*ProjectCandidate
	for _, c := range candidates {
		if _, found := existing[c.PlatformID]; !found {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

func registerConsumers(ctx context.Context) error {
	platform := system.Platform()
	subject := fmt.Sprintf("%s.%s", constants.SubjectScrapeRequested, platform)
	consumer := fmt.Sprintf("scout-%s", strings.ReplaceAll(platform, ".", "-"))

	if err := system.Subscribe(ctx, constants.StreamScrape, consumer, subject, handleScrapeRequested); err != nil {
		return fmt.Errorf("subscribe %s: %w", subject, err)
	}
	getLogger().Infof("subscribed → %s", subject)
	return nil
}

func handleScrapeRequested(data []byte) error {
	var event models.ScrapeRequestedEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal scrape.requested: %w", err)
	}
	getLogger().Infof("on-demand scrape: url=%s", event.URL)
	scrapeURL(context.Background(), event.URL)
	return nil
}

// scrapeURL handles an on-demand scrape for a single URL received via NATS.
func scrapeURL(ctx context.Context, url string) {
	platformID, slug := parseJobHref(url)
	candidate := &ProjectCandidate{
		PlatformID: platformID,
		Slug:       slug,
		URL:        url,
		Source:     Source,
		Current:    1,
		Total:      1,
	}
	processOneCandidate(ctx, candidate)
}

// processOneCandidate inspects a single candidate's detail page.
func processOneCandidate(ctx context.Context, c *ProjectCandidate) {
	log := getLogger().WithFields(logrus.Fields{
		"platform_id": c.PlatformID,
		"current":     c.Current,
		"total":       c.Total,
		"url":         c.URL,
	})

	inspector := &Inspector{URL: c.URL, PlatformID: c.PlatformID, Current: c.Current, Total: c.Total}
	result, err := inspector.Inspect()
	if err != nil {
		log.Warnf("inspect failed: %v — recording for retry", err)
		system.RecordError(ctx, models.ServiceError{
			ServiceName: constants.ServiceScout,
			ErrorName:   constants.ErrNameScrapeInspectFailed,
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
	p := models.NewPersistedProject(result, c.PlatformID, Source, time.Now(), "")
	p.PlatformUpdatedAt = c.PostedAt
	if err := system.GetStorage().Replace(ctx, constants.MongoProjectsCollection, p); err != nil {
		log.Errorf("replace project %s: %v", p.PlatformID, err)
		return
	}
	log.Infof("project saved with internal id %s", p.GetId())

	c.UpdateFromResult(result)

	subject := constants.SubjectProjectCreated
	if err := system.Publish(ctx, subject, p.GetEvent()); err != nil {
		log.Errorf("publish %s: %v", subject, err)
	} else {
		log.Infof("published %s for %s", subject, p.GetId())
	}
}
