package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/interfaces"
	"hblabs.co/falcon/modules/platformkit"
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

	SetLogger(logger any)

	SetSaveHandler(fn platformkit.SaveFn)
	SetFilterHandler(fn platformkit.FilterFn)
	SetWarnHandler(fn platformkit.WarnFn)
	SetErrHandler(fn platformkit.ErrFn)
	SetBatchConfig(cfg platformkit.BatchConfig)

	// BaseURL returns the platform's root URL (e.g. "https://www.redglobal.de")
	// used by the service to fetch well-known metadata files (robots.txt, etc.).
	// Return "" to opt out of metadata refresh.
	BaseURL() string

	// CompanyID returns the platform-assigned company id used to locate this
	// platform's company document in the companies collection. Required if
	// BaseURL is non-empty; the service log.Fatals at startup if missing.
	CompanyID() string

	// Init performs one-time setup: DB indexes, session login, etc.
	Init(ctx context.Context) error

	// Subscribe registers NATS consumers for on-demand scraping and admin triggers.
	StartConsumers(ctx context.Context) error

	// StartWorkers launches background goroutines (retry workers, etc.).
	StartWorkers(ctx context.Context)

	// Poll starts the main polling loop. Blocks until ctx is cancelled.
	Poll(ctx context.Context) func()
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
		p.SetSaveHandler(newSaveFn(logger))
		p.SetFilterHandler(newFilterFn(logger))
		p.SetWarnHandler(newWarnFn(p.Name()))
		p.SetErrHandler(newErrFn(p.Name()))

		batchCfg := system.BatchCfg()
		p.SetBatchConfig(platformkit.BatchConfig{
			Size:       batchCfg.Size,
			ItemDelay:  batchCfg.ItemDelay,
			BatchDelay: batchCfg.BatchDelay,
		})

		if err := p.Init(ctx); err != nil {
			logger.Fatalf("init: %v", err)
		}

		if err := p.StartConsumers(ctx); err != nil {
			logger.Fatalf("subscribe: %v", err)
		}

		go p.StartWorkers(ctx)
		go s.Poll(logger, p.Poll(ctx))

		// Spawn a low-frequency metadata refresh loop for every platform that
		// declares both a base URL and a company_id. The fetcher is generic
		// (well-known files always live in the same paths), so the service owns
		// the entire flow — runners stay focused on scraping.
		if p.BaseURL() != "" && p.CompanyID() != "" {
			go s.runMetadataLoop(ctx, p, logger)
		}

		logger.Info("platform registered and running")
	}

	system.Wait()
	logrus.Info("all scout platforms stopped")
}

// metadataRefreshInterval controls how often the service re-fetches well-known
// metadata files for each registered platform. Hourly is plenty — robots.txt
// and friends rarely change, but a fresh snapshot is useful when investigating
// drift incidents.
const metadataRefreshInterval = 1 * time.Hour

// runMetadataLoop periodically downloads well-known metadata files (robots.txt,
// security.txt, humans.txt) for the platform and persists them onto its company
// document. Runs once at startup, then every metadataRefreshInterval. Stops
// when ctx is cancelled.
//
// If the company is not found in the companies collection on the very first
// pass, the process is killed via Fatal — there is no sane default for "target
// company missing" and silently dropping the snapshot every cycle would hide
// the misconfiguration.
func (s *Service) runMetadataLoop(ctx context.Context, p Platform, logger *logrus.Entry) {
	companyID := p.CompanyID()
	baseURL := p.BaseURL()

	// Verify the target company exists once at startup. Fail loudly if not.
	var probe models.Company
	if err := system.GetStorage().GetByField(ctx, constants.MongoCompaniesCollection, "company_id", companyID, &probe); err != nil {
		logger.Fatalf("metadata loop: company with company_id=%q not found in companies collection: %v", companyID, err)
		return
	}

	platformName := p.Name()

	refresh := func() {
		// Re-fetch the company on every cycle so the diff against the freshly
		// downloaded files is against the latest persisted state, not against
		// a stale startup snapshot.
		var current models.Company
		if err := system.GetStorage().GetByField(ctx, constants.MongoCompaniesCollection, "company_id", companyID, &current); err != nil {
			logger.Errorf("metadata loop: re-fetch company %s failed: %v", companyID, err)
			return
		}

		files := fetchPlatformMetadata(ctx, baseURL)
		if len(files) == 0 {
			logger.Warnf("metadata loop: no files fetched for %s — skipping update", baseURL)
			return
		}

		next := models.CompanyMetadata{
			RobotsTxt:   files["robots.txt"],
			SecurityTxt: files["security.txt"],
			HumansTxt:   files["humans.txt"],
			SitemapURL:  files["sitemap_url"],
			UpdatedAt:   time.Now(),
		}

		// Compare against the previously stored snapshot. The first refresh
		// after deployment finds Metadata == nil and is treated as "no diff" —
		// we don't fire warnings for the initial seed, only for actual drift
		// once a baseline exists.
		changed := diffCompanyMetadata(current.Metadata, &next)
		for _, field := range changed {
			s.recordMetadataChangedWarning(ctx, platformName, companyID, field, current.Metadata, &next)
		}

		update := bson.M{
			"metadata":   next,
			"updated_at": time.Now(),
		}
		if err := system.GetStorage().Set(ctx, constants.MongoCompaniesCollection, bson.M{"company_id": companyID}, update); err != nil {
			logger.Errorf("metadata loop: update for company %s failed: %v", companyID, err)
			return
		}
		if len(changed) > 0 {
			logger.Warnf("metadata refreshed for company %s — changed: %v", companyID, changed)
		} else {
			logger.Infof("metadata refreshed for company %s (%d files, no changes)", companyID, len(files))
		}
	}

	refresh() // run once at startup

	ticker := time.NewTicker(metadataRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}

func (s *Service) Poll(logger interfaces.Logger, callback func()) {
	system.Poll(system.Ctx(), system.PollInterval(), logger, callback)
}

// missingMethods returns method names that iface requires but concrete does not implement.
func missingMethods(iface any, concrete any) []string {
	ifaceType := reflect.TypeOf(iface).Elem()
	concreteType := reflect.TypeOf(concrete)

	var missing []string
	for i := range ifaceType.NumMethod() {
		m := ifaceType.Method(i)
		if _, ok := concreteType.MethodByName(m.Name); !ok {
			missing = append(missing, m.Name)
		}
	}
	return missing
}

func newSaveFn(logger *logrus.Entry) platformkit.SaveFn {
	return func(ctx context.Context, project any, existing any) error {
		src, ok := project.(interfaces.Project)
		if !ok {
			missing := missingMethods((*interfaces.Project)(nil), project)
			logger.Errorf("project does not implement interfaces.Project — missing: %v", missing)
			return fmt.Errorf("invalid project type")
		}

		var prev *models.PersistedProject
		if existing != nil {
			prev, _ = existing.(*models.PersistedProject)
		}

		p := models.NewPersistedProject(src, prev)

		if err := system.GetStorage().Replace(ctx, constants.MongoProjectsCollection, p); err != nil {
			logger.Errorf("replace project %s: %v", p.PlatformID, err)
			return err
		}
		logger.Infof("project saved with internal id %s", p.GetId())

		subject := constants.SubjectProjectCreated
		if err := system.Publish(ctx, subject, p.GetEvent()); err != nil {
			logger.Errorf("publish %s: %v", subject, err)
		} else {
			logger.Infof("published %s for %s", subject, p.GetId())
		}

		return nil
	}
}

func newFilterFn(_ *logrus.Entry) platformkit.FilterFn {
	return func(
		ctx context.Context,
		platform string,
		updatedAt map[string]time.Time,
	) (map[string]bool, map[string]any, error) {
		platformIDs := make([]string, 0, len(updatedAt))
		for id := range updatedAt {
			platformIDs = append(platformIDs, id)
		}

		// Check which candidates already exist in the projects collection.
		var existing []models.PersistedProject
		if err := system.GetStorage().GetMany(ctx, constants.MongoProjectsCollection, bson.M{
			"platform":    platform,
			"platform_id": bson.M{"$in": platformIDs},
		}, &existing); err != nil {
			return nil, nil, err
		}

		// Check which candidates have pending scrape errors.
		var pendingErrors []models.ServiceError
		if err := system.GetStorage().GetMany(ctx, constants.MongoErrorsCollection, bson.M{
			"service_name": constants.ServiceScout,
			"platform":     platform,
			"platform_id":  bson.M{"$in": platformIDs},
			// "error_name": bson.M{"$in": []string{
			// 	constants.ErrNameScrapeInspectFailed,
			// 	constants.ErrNameScrapeServerError,
			// }},
		}, &pendingErrors); err != nil {
			return nil, nil, err
		}

		skip := make(map[string]bool, len(existing)+len(pendingErrors))
		existingByID := make(map[string]any, len(existing))

		// Skip candidates with pending errors.
		for _, e := range pendingErrors {
			skip[e.PlatformID] = true
		}

		// Skip candidates that exist and haven't changed.
		// Pass through the existing record so the runner can hand it back to SaveFn,
		// avoiding a second DB round-trip when re-scraping a changed item.
		for i := range existing {
			p := &existing[i]
			existingByID[p.PlatformID] = p

			candidateUpdatedAt, ok := updatedAt[p.PlatformID]
			if ok && sameInstant(p.PlatformUpdatedAt, candidateUpdatedAt) {
				skip[p.PlatformID] = true
			}
		}

		return skip, existingByID, nil
	}
}

// sameInstant compares two times for equality after normalizing both to UTC.
// Returns true if a and b represent the same moment in time.
func sameInstant(a, b time.Time) bool {
	return a.UTC().Equal(b.UTC())
}

// newWarnFn returns a WarnFn that persists warnings to the warnings collection,
// pre-tagged with the scout service name and the platform that emitted them.
//
// Mirrors newErrFn: warnings with priority "high" or "critical" are also
// published to signal.admin_alert so falcon-signal can notify the operations
// team. The published event carries only the warning id + kind — signal loads
// the full record from MongoDB and never sees the HTML through NATS.
func newWarnFn(platform string) platformkit.WarnFn {
	return func(ctx context.Context, name, message, priority, html string, candidate any) error {
		warnID := system.RecordWarning(ctx, models.ServiceWarning{
			ServiceName: constants.ServiceScout,
			WarningName: name,
			Message:     message,
			Priority:    models.WarningPriority(priority),
			Platform:    platform,
			HTML:        html,
			Candidate:   candidate,
		})

		if warnID != "" && shouldEscalateToAdmins(priority) {
			evt := models.AdminAlertEvent{
				Kind: models.AdminAlertKindWarning,
				ID:   warnID,
			}
			if pubErr := system.Publish(ctx, constants.SubjectSignalAdminAlert, evt); pubErr != nil {
				logrus.Errorf("publish admin alert for warning %s: %v", warnID, pubErr)
			}
		}

		return nil
	}
}

// newErrFn returns an ErrFn that persists errors to the errors collection,
// pre-tagged with the scout service name and the platform that emitted them.
// The runner classifies the error via platformkit.ClassifyError before calling.
// Identity fields (platform_id, url) live inside the candidate; queries can use
// nested paths like db.errors.find({"candidate.platform_id": "..."}).
//
// As a side effect, errors with priority "high" or "critical" are also
// published to signal.admin_alert so falcon-signal can fan them out to the
// operations team via email + push. The published event carries only the
// error ID — signal loads the full record from MongoDB. There is no dedup
// here yet, so high/critical events MUST be rare for this to be sustainable.
func newErrFn(platform string) platformkit.ErrFn {
	return func(ctx context.Context, name, message, priority, html string, candidate any) error {
		errID := system.RecordError(ctx, models.ServiceError{
			ServiceName: constants.ServiceScout,
			ErrorName:   name,
			Error:       message,
			Priority:    models.ErrorPriority(priority),
			Platform:    platform,
			HTML:        html,
			Candidate:   candidate,
		})

		if errID != "" && shouldEscalateToAdmins(priority) {
			evt := models.AdminAlertEvent{
				Kind: models.AdminAlertKindError,
				ID:   errID,
			}
			if pubErr := system.Publish(ctx, constants.SubjectSignalAdminAlert, evt); pubErr != nil {
				logrus.Errorf("publish admin alert for error %s: %v", errID, pubErr)
			}
		}

		return nil
	}
}

// shouldEscalateToAdmins reports whether an error priority warrants paging
// the operations team. Today only "high" and "critical" qualify; "low" and
// "medium" go to the errors collection without notification.
func shouldEscalateToAdmins(priority string) bool {
	switch models.ErrorPriority(priority) {
	case models.ErrorPriorityHigh, models.ErrorPriorityCritical:
		return true
	}
	return false
}

// diffCompanyMetadata returns the names of fields that differ between the
// previously stored snapshot and the freshly fetched one. Returns nil on the
// very first refresh (prev == nil) so the initial seed of the data does not
// fire warnings — we only care about drift from an established baseline.
//
// The UpdatedAt field is intentionally excluded since it changes every cycle.
func diffCompanyMetadata(prev, next *models.CompanyMetadata) []string {
	if prev == nil || next == nil {
		return nil
	}
	var changed []string
	if prev.RobotsTxt != next.RobotsTxt {
		changed = append(changed, "robots.txt")
	}
	if prev.SecurityTxt != next.SecurityTxt {
		changed = append(changed, "security.txt")
	}
	if prev.HumansTxt != next.HumansTxt {
		changed = append(changed, "humans.txt")
	}
	if prev.SitemapURL != next.SitemapURL {
		changed = append(changed, "sitemap_url")
	}
	return changed
}

// recordMetadataChangedWarning persists a warning per changed metadata file
// and publishes an admin alert so the operations team is notified. The HTML
// snapshot is the new content of the file (not the diff) so investigators can
// see exactly what's there now; the previous version remains in the company
// document until the upcoming Set call overwrites it.
func (s *Service) recordMetadataChangedWarning(
	ctx context.Context,
	platform, companyID, field string,
	prev *models.CompanyMetadata,
	next *models.CompanyMetadata,
) {
	message := fmt.Sprintf("metadata file %q for company %s changed since last refresh", field, companyID)
	candidate := map[string]any{
		"company_id": companyID,
		"field":      field,
		"previous":   metadataFieldValue(prev, field),
	}
	priority := string(models.WarningPriorityHigh)

	warnID := system.RecordWarning(ctx, models.ServiceWarning{
		ServiceName: constants.ServiceScout,
		WarningName: platformkit.WarnCompanyMetadataChanged,
		Message:     message,
		Priority:    models.WarningPriority(priority),
		Platform:    platform,
		HTML:        metadataFieldValue(next, field),
		Candidate:   candidate,
	})

	if warnID != "" && shouldEscalateToAdmins(priority) {
		evt := models.AdminAlertEvent{
			Kind: models.AdminAlertKindWarning,
			ID:   warnID,
		}
		if pubErr := system.Publish(ctx, constants.SubjectSignalAdminAlert, evt); pubErr != nil {
			logrus.Errorf("publish admin alert for metadata-change warning %s: %v", warnID, pubErr)
		}
	}
}

// metadataFieldValue returns the string value of a CompanyMetadata field by
// the same name used in diffCompanyMetadata. Empty string when the snapshot
// is nil or the field name is unknown.
func metadataFieldValue(m *models.CompanyMetadata, field string) string {
	if m == nil {
		return ""
	}
	switch field {
	case "robots.txt":
		return m.RobotsTxt
	case "security.txt":
		return m.SecurityTxt
	case "humans.txt":
		return m.HumansTxt
	case "sitemap_url":
		return m.SitemapURL
	}
	return ""
}
