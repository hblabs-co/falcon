package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	environment "hblabs.co/falcon/common/environment"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/platformkit"
)

// Retry defaults — overridable via env vars.
const (
	defaultRetryInspectInterval = 5 * time.Minute
	defaultRetryServerInterval  = 30 * time.Minute
	defaultMaxRetryAttempts     = 50
)

// startRetryWorkers launches two background workers per platform:
//   - Inspect failures (scrape_inspect_failed): retried every RETRY_INSPECT_INTERVAL (default 5m)
//   - Server errors (scrape_server_error): retried every RETRY_SERVER_INTERVAL (default 30m)
//
// Both workers share the same processRetryBatch logic; only the error_name
// filter and interval differ.
func startRetryWorkers(ctx context.Context, p Platform, logger *logrus.Entry) {
	inspectInterval := environment.ParseDuration("RETRY_INSPECT_INTERVAL", defaultRetryInspectInterval.String())
	serverInterval := environment.ParseDuration("RETRY_SERVER_INTERVAL", defaultRetryServerInterval.String())
	maxAttempts := environment.ParseInt("MAX_RETRY_ATTEMPTS", defaultMaxRetryAttempts)

	logger.Infof("[retry] workers started — inspect every %s, server every %s, max %d attempts",
		inspectInterval, serverInterval, maxAttempts)

	system.StartWorker(ctx, inspectInterval, func(ctx context.Context) {
		processRetryBatch(ctx, p, logger, platformkit.ErrNameScrapeInspectFailed, maxAttempts)
	})
	system.StartWorker(ctx, serverInterval, func(ctx context.Context) {
		processRetryBatch(ctx, p, logger, platformkit.ErrNameScrapeServerError, maxAttempts)
	})
}

// processRetryBatch queries the errors collection for per-item failures that
// are eligible for retry (same platform, correct error_name, candidate present,
// retry_count < max) and retries each unique platform_id once.
func processRetryBatch(ctx context.Context, p Platform, logger *logrus.Entry, errorName string, maxAttempts int) {
	platform := p.Name()

	var errors []models.ServiceError
	filter := bson.M{
		"service_name": constants.ServiceScout,
		"platform":     platform,
		"error_name":   errorName,
		"candidate":    bson.M{"$exists": true},
		"resolved":     bson.M{"$ne": true},
		"$or": bson.A{
			bson.M{"retry_count": bson.M{"$lt": maxAttempts}},
			bson.M{"retry_count": bson.M{"$exists": false}},
		},
	}
	if err := system.GetStorage().GetMany(ctx, constants.MongoErrorsCollection, filter, &errors); err != nil {
		logger.Errorf("[retry:%s] query errors: %v", errorName, err)
		return
	}
	if len(errors) == 0 {
		return
	}

	// Deduplicate by platform_id — keep the one with the highest retry_count.
	// Multiple error records for the same job can accumulate if the job fails
	// across several poll cycles before the retry worker gets to it.
	type candidateKey struct {
		platformID string
	}
	byPlatformID := make(map[string]models.ServiceError, len(errors))
	for _, e := range errors {
		if e.PlatformID == "" && e.Candidate == nil {
			continue // categorical error — not retryable
		}
		pid := extractPlatformID(e)
		if pid == "" {
			continue
		}
		if existing, ok := byPlatformID[pid]; !ok || e.RetryCount > existing.RetryCount {
			byPlatformID[pid] = e
		}
	}

	logger.Infof("[retry:%s] %d error(s) for %d unique project(s)", errorName, len(errors), len(byPlatformID))

	for _, svcErr := range byPlatformID {
		retryOneError(ctx, p, logger, svcErr, maxAttempts)
	}
}

// retryOneError attempts to re-process a single failed candidate via the
// platform's Retry method. On success it deletes all related error records;
// on failure it increments retry_count and escalates to critical if the max
// is reached.
func retryOneError(ctx context.Context, p Platform, logger *logrus.Entry, svcErr models.ServiceError, maxAttempts int) {
	pid := extractPlatformID(svcErr)
	log := logger.WithFields(logrus.Fields{
		"error_id":    svcErr.ID,
		"platform_id": pid,
		"error_name":  svcErr.ErrorName,
		"attempt":     svcErr.RetryCount + 1,
	})

	// Load existing project for this platform_id (if it was saved before).
	// Passed to Retry so the runner can preserve the nanoid via SaveFn.
	var existing *models.PersistedProject
	if pid != "" {
		var probe models.PersistedProject
		if err := system.GetStorage().Get(ctx, constants.MongoProjectsCollection, bson.M{
			"platform_id": pid,
			"platform":    p.Name(),
		}, &probe); err == nil {
			existing = &probe
		}
	}

	retryErr := p.Retry(ctx, svcErr.Candidate, existing)

	if retryErr == nil {
		// Success — delete ALL error records for this platform_id on this platform.
		// The platform_id may live at top-level OR inside the candidate sub-doc
		// depending on how the error was recorded — match either.
		if delErr := system.GetStorage().DeleteMany(ctx, constants.MongoErrorsCollection, bson.M{
			"platform": p.Name(),
			"$or": bson.A{
				bson.M{"platform_id": pid},
				bson.M{"candidate.platform_id": pid},
			},
		}); delErr != nil {
			log.Warnf("[retry] cleanup errors: %v", delErr)
		}
		log.Infof("[retry] succeeded on attempt %d — errors removed", svcErr.RetryCount+1)
		return
	}

	// 410 Gone — project permanently removed, delete the error record.
	if platformkit.IsGone(retryErr) {
		log.Infof("[retry] project gone (410) — removing error")
		_ = system.GetStorage().DeleteByField(ctx, constants.MongoErrorsCollection, "id", svcErr.ID)
		return
	}

	// Still failing — increment retry_count.
	newCount := svcErr.RetryCount + 1
	update := bson.M{
		"retry_count": newCount,
		"error":       retryErr.Error(),
		"occurred_at": time.Now(),
	}
	if newCount >= maxAttempts {
		update["priority"] = models.ErrorPriorityCritical
		log.Warnf("[retry] max retries reached — escalating to critical")
	}
	_ = system.GetStorage().SetById(ctx, constants.MongoErrorsCollection, svcErr.ID, update)

	// Notify admins when max retries is reached — the issue has been persisting
	// long enough to warrant human attention. The alert points at the existing
	// error doc (now updated with priority=critical) so signal can load the
	// full context.
	if newCount >= maxAttempts {
		publishAdminAlert(ctx, models.AdminAlertKindError, svcErr.ID)
	}

	log.Warnf("[retry] attempt %d failed: %v", newCount, retryErr)
}

// extractPlatformID tries to read platform_id from the error's top-level
// PlatformID field. If empty (because we dropped it from the model), it
// falls back to extracting it from the Candidate stored by MongoDB.
//
// The candidate comes back from the BSON driver as different types depending
// on driver version and configuration (bson.M, bson.D, map[string]any, etc.)
// so we use a json round-trip to normalize it into a simple map.
func extractPlatformID(e models.ServiceError) string {
	if e.PlatformID != "" {
		return e.PlatformID
	}
	if e.Candidate == nil {
		return ""
	}

	data, err := json.Marshal(e.Candidate)
	if err != nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	if pid, ok := m["platform_id"].(string); ok {
		return pid
	}
	return ""
}
