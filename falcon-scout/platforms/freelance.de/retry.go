package freelancede

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

const maxRetryAttempts = 50

// StartRetryWorker launches two background workers:
// - Every 5 min: retries normal scrape failures (scrape_inspect_failed)
// - Every 30 min: retries server errors (scrape_server_error) which need longer cooldown
func StartRetryWorker(ctx context.Context) {
	getLogger().Info("[retry] scout retry workers started")
	system.StartWorker(ctx, 5*time.Minute, retryNormalErrors)
	system.StartWorker(ctx, 30*time.Minute, retryServerErrors)
}

func retryNormalErrors(ctx context.Context) {
	processRetryBatch(ctx, constants.ErrNameScrapeInspectFailed)
}

func retryServerErrors(ctx context.Context) {
	processRetryBatch(ctx, constants.ErrNameScrapeServerError)
}

func processRetryBatch(ctx context.Context, errorName string) {
	var errors []models.ServiceError
	filter := bson.M{
		"service_name": constants.ServiceScout,
		"platform":     Source,
		"error_name":   errorName,
		"candidate":    bson.M{"$exists": true},
		"$or": bson.A{
			bson.M{"retry_count": bson.M{"$lt": maxRetryAttempts}},
			bson.M{"retry_count": bson.M{"$exists": false}},
		},
	}
	if err := system.GetStorage().GetMany(ctx, constants.MongoErrorsCollection, filter, &errors); err != nil {
		getLogger().Errorf("[retry:%s] query errors: %v", errorName, err)
		return
	}

	if len(errors) == 0 {
		return
	}

	// Deduplicate by platform_id — keep the one with the highest retry_count.
	byPlatformID := make(map[string]models.ServiceError, len(errors))
	for _, e := range errors {
		if e.PlatformID == "" {
			continue
		}
		if existing, ok := byPlatformID[e.PlatformID]; !ok || e.RetryCount > existing.RetryCount {
			byPlatformID[e.PlatformID] = e
		}
	}

	getLogger().Infof("[retry:%s] %d error(s) for %d unique project(s)", errorName, len(errors), len(byPlatformID))

	for _, svcErr := range byPlatformID {
		retryOneError(ctx, svcErr)
	}
}

func retryOneError(ctx context.Context, svcErr models.ServiceError) {
	log := getLogger().WithFields(logrus.Fields{
		"error_id":    svcErr.ID,
		"platform_id": svcErr.PlatformID,
		"error_name":  svcErr.ErrorName,
		"attempt":     svcErr.RetryCount + 1,
	})

	candidate, err := decodeCandidate(svcErr.Candidate)
	if err != nil {
		log.Warnf("[retry] cannot decode candidate: %v — skipping", err)
		return
	}

	log.Info("[retry] retrying failed scrape")

	inspector := &Inspector{Url: candidate.URL, PlatformID: candidate.PlatformID, Current: 1, Total: 1}
	result, inspectErr := inspector.Inspect()
	if inspectErr != nil {
		// 410 Gone — project permanently removed, delete the error.
		if errors.Is(inspectErr, ErrGone) {
			log.Infof("[retry] project gone (410) — removing error")
			_ = system.GetStorage().DeleteByField(ctx, constants.MongoErrorsCollection, "id", svcErr.ID)
			return
		}

		newCount := svcErr.RetryCount + 1
		update := bson.M{
			"retry_count": newCount,
			"error":       inspectErr.Error(),
			"occurred_at": time.Now(),
		}
		if newCount >= maxRetryAttempts {
			update["priority"] = models.ErrorPriorityCritical
			log.Warnf("[retry] max retries reached — escalating to critical")
		}
		_ = system.GetStorage().SetById(ctx, constants.MongoErrorsCollection, svcErr.ID, update)
		log.Warnf("[retry] attempt %d failed: %v", newCount, inspectErr)
		return
	}

	// Success — save project and delete all errors for this platform_id.
	saveProject(ctx, candidate, result, log)
	if err := system.GetStorage().DeleteMany(ctx, constants.MongoErrorsCollection, bson.M{
		"platform_id": svcErr.PlatformID,
		"platform":    Source,
	}); err != nil {
		log.Warnf("[retry] cleanup errors: %v", err)
	}
	log.Infof("[retry] scrape succeeded on attempt %d — errors removed", svcErr.RetryCount+1)
}

func decodeCandidate(raw any) (*ProjectCandidate, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var c ProjectCandidate
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}
