package project

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

const (
	retryInterval = 1 * time.Minute
	maxRetries    = 5
)

var retryableErrorNames = []string{
	constants.ErrNameNormalizerLLMParse,
	constants.ErrNameNormalizerTranslateParse,
}

func (s *Service) startRetryWorker(ctx context.Context) {
	logrus.Info("[project-retry] worker started")
	system.StartWorker(ctx, retryInterval, s.processRetryBatch)
}

func (s *Service) processRetryBatch(ctx context.Context) {
	var errors []models.ServiceError
	filter := bson.M{
		"error_name": bson.M{"$in": retryableErrorNames},
		"resolved":   bson.M{"$ne": true},
		"$or": bson.A{
			bson.M{"retry_count": bson.M{"$lt": maxRetries}},
			bson.M{"retry_count": bson.M{"$exists": false}},
		},
	}
	if err := system.GetStorage().GetMany(ctx, constants.MongoErrorsCollection, filter, &errors); err != nil {
		logrus.Errorf("[project-retry] query errors: %v", err)
		return
	}

	if len(errors) == 0 {
		return
	}

	byProject := make(map[string]models.ServiceError, len(errors))
	for _, e := range errors {
		if e.ProjectID == "" {
			continue
		}
		if existing, ok := byProject[e.ProjectID]; !ok || e.RetryCount > existing.RetryCount {
			byProject[e.ProjectID] = e
		}
	}

	logrus.Infof("[project-retry] %d error(s) for %d unique project(s)", len(errors), len(byProject))

	first := true
	for _, svcErr := range byProject {
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
		"project_id": svcErr.ProjectID,
		"attempt":    svcErr.RetryCount + 1,
	})

	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, svcErr.ProjectID, &project); err != nil {
		log.Warnf("[project-retry] project not found, marking resolved: %v", err)
		markResolved(ctx, svcErr.ID)
		return
	}

	log.Info("[project-retry] retrying normalization")

	projectJSON, _ := json.Marshal(project)
	userPrompt := strings.ReplaceAll(
		"Extract and normalize the following project JSON according to your instructions.\nRespond ONLY with the JSON object (no language wrapper keys). No markdown, no explanation.\n\n{{project_json}}",
		"{{project_json}}", string(projectJSON))

	deContent, rawDE, normErr := s.llm.NormalizeDE(ctx, s.normalizePrompt, userPrompt, project.ID)
	if normErr != nil {
		updateRetry(ctx, svcErr, normErr, rawDE)
		log.Warnf("[project-retry] attempt %d failed: %v", svcErr.RetryCount+1, normErr)
		return
	}

	en, es, rawTranslate, transErr := s.llm.TranslateToEnEs(ctx, deContent, map[string]any{
		"project_id": project.ID,
		"platform":   project.Platform,
	})
	if transErr != nil {
		updateRetry(ctx, svcErr, transErr, rawTranslate)
		log.Warnf("[project-retry] attempt %d failed: %v", svcErr.RetryCount+1, transErr)
		return
	}

	if authoritativeCompany := resolveCompanyName(ctx, &project); authoritativeCompany != "" {
		overrideCompanyName(deContent, authoritativeCompany)
		overrideCompanyName(en, authoritativeCompany)
		overrideCompanyName(es, authoritativeCompany)
	}

	doc := &NormalizedProject{
		ProjectID:        project.ID,
		CompanyID:        project.CompanyID,
		DisplayUpdatedAt: project.DisplayUpdatedAt,
		En:               en,
		De:               deContent,
		Es:               es,
		NormalizedAt:     time.Now(),
		LLMModel:         s.llm.Model,
		LLMProvider:      s.llm.Provider,
	}

	if err := system.GetStorage().Set(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": project.ID}, doc); err != nil {
		log.Errorf("[project-retry] save: %v", err)
		return
	}

	evt := models.ProjectNormalizedEvent{
		ProjectID: project.ID,
		Platform:  project.Platform,
		Title:     project.Title,
	}
	if err := system.Publish(ctx, constants.SubjectProjectNormalized, evt); err != nil {
		log.Warnf("[project-retry] publish: %v", err)
	}

	resolveAllForProject(ctx, svcErr.ProjectID)
	log.Infof("[project-retry] succeeded on attempt %d", svcErr.RetryCount+1)
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

func resolveAllForProject(ctx context.Context, projectID string) {
	var errors []models.ServiceError
	if err := system.GetStorage().GetAllByField(ctx, constants.MongoErrorsCollection, "project_id", projectID, &errors); err != nil {
		return
	}
	for _, e := range errors {
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
