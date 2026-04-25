package match

import (
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

const (
	retryInterval = 1 * time.Minute
	maxRetries    = 5
)

var retryableErrorNames = []string{
	constants.ErrNameMatchLLMFailed,
}

func (s *Service) startRetryWorker(ctx context.Context) {
	logrus.Info("[match-retry] worker started")
	system.StartWorker(ctx, retryInterval, s.processRetryBatch)
}

func (s *Service) processRetryBatch(ctx context.Context) {
	var errs []models.ServiceError
	filter := bson.M{
		"service_name": constants.ServiceMatchEngine,
		"error_name":   bson.M{"$in": retryableErrorNames},
		"resolved":     bson.M{"$ne": true},
		"$or": bson.A{
			bson.M{"retry_count": bson.M{"$lt": maxRetries}},
			bson.M{"retry_count": bson.M{"$exists": false}},
		},
	}
	if err := system.GetStorage().GetMany(ctx, constants.MongoErrorsCollection, filter, &errs); err != nil {
		logrus.Errorf("[match-retry] query errors: %v", err)
		return
	}

	if len(errs) == 0 {
		return
	}

	// Dedup by (cv_id, project_id): keep the record with the highest retry_count
	// so the attempt counter doesn't reset after crashes leaving multiple entries.
	type pairKey struct{ cvID, projectID string }
	byPair := make(map[pairKey]models.ServiceError, len(errs))
	for _, e := range errs {
		if e.CVID == "" || e.ProjectID == "" {
			continue
		}
		k := pairKey{e.CVID, e.ProjectID}
		if existing, ok := byPair[k]; !ok || e.RetryCount > existing.RetryCount {
			byPair[k] = e
		}
	}

	logrus.Infof("[match-retry] %d error(s) for %d unique pair(s)", len(errs), len(byPair))

	first := true
	for _, svcErr := range byPair {
		if !first {
			time.Sleep(10 * time.Second)
		}
		first = false
		s.retryOne(ctx, svcErr)
	}
}

func (s *Service) retryOne(ctx context.Context, svcErr models.ServiceError) {
	log := logrus.WithFields(logrus.Fields{
		"error_id":   svcErr.ID,
		"cv_id":      svcErr.CVID,
		"project_id": svcErr.ProjectID,
		"attempt":    svcErr.RetryCount + 1,
	})

	var cv models.PersistedCV
	if err := system.GetStorage().GetById(ctx, constants.MongoCVsCollection, svcErr.CVID, &cv); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Warn("[match-retry] cv gone, marking resolved")
			markResolved(ctx, svcErr.ID)
			return
		}
		log.Warnf("[match-retry] fetch cv failed: %v", err)
		return
	}
	if cv.ExtractedText == "" {
		log.Warn("[match-retry] cv has no extracted text, marking resolved")
		markResolved(ctx, svcErr.ID)
		return
	}

	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, svcErr.ProjectID, &project); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Warn("[match-retry] project gone, marking resolved")
			markResolved(ctx, svcErr.ID)
			return
		}
		log.Warnf("[match-retry] fetch project failed: %v", err)
		return
	}

	log.Info("[match-retry] retrying scoring")

	// Use the normalized display title when available — same cleaning
	// as the primary scoring path in service.go.
	displayTitle := cleanProjectTitle(ctx, svcErr.ProjectID, project.Title)

	result, rawContent, err := s.scorer.Score(ctx, displayTitle, project.Description, cv.ExtractedText, logrus.Fields{
		"cv_id":      svcErr.CVID,
		"project_id": svcErr.ProjectID,
	})
	if err != nil {
		updateRetry(ctx, svcErr, err, rawContent)
		log.Warnf("[match-retry] attempt %d failed: %v", svcErr.RetryCount+1, err)
		return
	}

	result.CVID = svcErr.CVID
	result.UserID = svcErr.UserID
	result.ProjectID = svcErr.ProjectID
	// Same fallback chain as service.go: normalized → LLM-cleaned → raw.
	if displayTitle == project.Title && result.ProjectTitle != "" {
		// keep LLM-cleaned result.ProjectTitle
	} else {
		result.ProjectTitle = displayTitle
	}
	result.Platform = svcErr.Platform
	company, _ := system.GetCachedCompany(ctx, &project)
	result.CompanyName = company.CompanyName
	result.CompanyLogoURL = company.LogoMinioURL
	result.PassedThreshold = result.Score >= s.threshold
	result.Normalized = isProjectNormalized(ctx, svcErr.ProjectID)
	result.ScoredAt = time.Now()

	filter := bson.M{"cv_id": svcErr.CVID, "project_id": svcErr.ProjectID}
	if err := system.GetStorage().Set(ctx, constants.MongoMatchResultsCollection, filter, result); err != nil {
		log.Errorf("[match-retry] save: %v", err)
		return
	}

	resolveAllForPair(ctx, svcErr.CVID, svcErr.ProjectID)

	// Mirror the primary path's logging (service.go) so retry successes
	// are diffable against normal-flow lines — only the attempt= field
	// marks them apart.
	if !result.PassedThreshold {
		log.Infof("[match-retry] score %.1f below threshold %.1f — saved, not forwarded", result.Score, s.threshold)
		return
	}

	if err := system.Publish(ctx, constants.SubjectMatchResult, result); err != nil {
		log.Warnf("[match-retry] publish: %v", err)
		return
	}

	log.Infof("[match-retry] published match.result — score %.1f label=%s", result.Score, result.Label)
	log.Infof("[match-retry] succeeded on attempt %d (score=%.1f)", svcErr.RetryCount+1, result.Score)
}

func updateRetry(ctx context.Context, svcErr models.ServiceError, err error, rawContent string) {
	newCount := svcErr.RetryCount + 1
	update := bson.M{
		"retry_count": newCount,
		"error":       err.Error(),
		"occurred_at": time.Now(),
	}
	if newCount >= maxRetries {
		update["priority"] = models.ErrorPriorityCritical
	} else {
		update["priority"] = escalatePriority(svcErr.Priority)
	}
	_ = system.GetStorage().SetById(ctx, constants.MongoErrorsCollection, svcErr.ID, update)
	if rawContent != "" {
		_ = system.GetStorage().SetById(ctx, constants.MongoErrorsCollection, svcErr.ID, bson.M{
			"raw_llm_content": rawContent,
		})
	}
}

func markResolved(ctx context.Context, errorID string) {
	_ = system.GetStorage().SetById(ctx, constants.MongoErrorsCollection, errorID, bson.M{"resolved": true})
}

// resolveAllForPair marks every unresolved error for this (cv_id, project_id)
// as resolved once any one of them succeeds on retry.
func resolveAllForPair(ctx context.Context, cvID, projectID string) {
	var errs []models.ServiceError
	filter := bson.M{
		"service_name": constants.ServiceMatchEngine,
		"cv_id":        cvID,
		"project_id":   projectID,
	}
	if err := system.GetStorage().GetMany(ctx, constants.MongoErrorsCollection, filter, &errs); err != nil {
		return
	}
	for _, e := range errs {
		if !e.Resolved {
			markResolved(ctx, e.ID)
		}
	}
}

func escalatePriority(current models.ErrorPriority) models.ErrorPriority {
	switch current {
	case models.ErrorPriorityLow:
		return models.ErrorPriorityMedium
	case models.ErrorPriorityMedium:
		return models.ErrorPriorityHigh
	default:
		return models.ErrorPriorityHigh
	}
}
