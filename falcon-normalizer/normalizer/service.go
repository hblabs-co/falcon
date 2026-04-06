package normalizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// NormalizedProject is the document stored in the projects_normalized collection.
type NormalizedProject struct {
	ProjectID         string         `json:"project_id"          bson:"project_id"`
	Platform          string         `json:"platform"            bson:"platform"`
	PlatformUpdatedAt string         `json:"platform_updated_at" bson:"platform_updated_at"`
	CompanyName       string         `json:"company_name"        bson:"company_name"`
	En                map[string]any `json:"en"                  bson:"en"`
	De                map[string]any `json:"de"                  bson:"de"`
	Es                map[string]any `json:"es"                  bson:"es"`
	NormalizedAt      time.Time      `json:"normalized_at"       bson:"normalized_at"`
	LLMModel          string         `json:"llm_model"           bson:"llm_model"`
	LLMProvider       string         `json:"llm_provider"        bson:"llm_provider"`
}

// Service consumes project.created and project.updated events, calls the LLM
// to produce a trilingual normalized document, and publishes project.normalized.
type Service struct {
	llm *llmClient
}

func NewService(ctx context.Context, normalizePrompt, translatePrompt string) (*Service, error) {

	if err := system.GetStorage().EnsureIndex(ctx, system.NewIndexSpec(
		constants.MongoNormalizedProjectsCollection, "project_id", true,
	)); err != nil {
		logrus.Fatalf("ensure index: %v", err)
	}

	if err := system.GetStorage().EnsureIndex(ctx, system.NewIndexSpec(
		constants.MongoNormalizedProjectsCollection, "platform_updated_at", false,
	)); err != nil {
		logrus.Fatalf("ensure index platform_updated_at: %v", err)
	}

	llm, err := newLLMClient(normalizePrompt, translatePrompt)
	if err != nil {
		return nil, fmt.Errorf("llm client: %w", err)
	}

	return &Service{llm: llm}, nil
}

// Run subscribes to project.created and project.updated and blocks until ctx is cancelled.
func (s *Service) Run() error {
	ctx := system.Ctx()

	if err := system.Subscribe(ctx, constants.StreamProjects, "normalizer-created", constants.SubjectProjectCreated, s.handleEvent); err != nil {
		return fmt.Errorf("subscribe project.created: %w", err)
	}
	if err := system.Subscribe(ctx, constants.StreamProjects, "normalizer-updated", constants.SubjectProjectUpdated, s.handleEvent); err != nil {
		return fmt.Errorf("subscribe project.updated: %w", err)
	}

	logrus.Info("normalizer subscribed to project.created and project.updated")
	system.Wait()
	return nil
}

func (s *Service) handleEvent(data []byte) error {
	var event models.ProjectEvent
	if err := json.Unmarshal(data, &event); err != nil {
		// Malformed event — ack it so it never retries.
		logrus.Errorf("unmarshal project event: %v (dropping message)", err)
		return nil
	}

	log := logrus.WithFields(logrus.Fields{
		"project_id": event.ProjectID,
		"platform":   event.Platform,
	})

	ctx := context.Background()

	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, event.ProjectID, &project); err != nil {
		return fmt.Errorf("fetch project %s: %w", event.ProjectID, err)
	}

	// Idempotency: skip if this exact version is already normalized.
	var existing NormalizedProject
	if err := system.GetStorage().Get(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": project.ID, "platform_updated_at": project.PlatformUpdatedAt},
		&existing,
	); err == nil {
		log.Info("project already normalized for this version, skipping")
		return nil
	}

	log.Info("normalizing project")

	raw, rawContent, llmErr := s.llm.Normalize(ctx, &project)
	if llmErr != nil {
		errName := constants.ErrNameNormalizerLLMParse
		if strings.Contains(llmErr.Error(), "translate") {
			errName = constants.ErrNameNormalizerTranslateParse
		}
		s.saveError(ctx, &project, errName, llmErr, rawContent)
		return nil // ack — do not redeliver into infinite LLM loop
	}

	doc := &NormalizedProject{
		ProjectID:         project.ID,
		Platform:          project.Platform,
		PlatformUpdatedAt: project.PlatformUpdatedAt,
		CompanyName:       project.Company,
		En:                raw.En,
		De:                raw.De,
		Es:                raw.Es,
		NormalizedAt:      time.Now(),
		LLMModel:          s.llm.model,
		LLMProvider:       s.llm.provider,
	}

	if err := system.GetStorage().Set(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": project.ID}, doc); err != nil {
		return fmt.Errorf("save normalized project: %w", err)
	}

	evt := models.ProjectNormalizedEvent{
		ProjectID: project.ID,
		Platform:  project.Platform,
		Title:     project.Title,
	}
	if err := system.Publish(ctx, constants.SubjectProjectNormalized, evt); err != nil {
		return fmt.Errorf("publish project.normalized: %w", err)
	}

	log.Infof("project normalized and published — en_keys=%d de_keys=%d es_keys=%d",
		len(raw.En), len(raw.De), len(raw.Es))
	return nil
}

func (s *Service) saveError(ctx context.Context, project *models.PersistedProject, errName string, normErr error, rawContent string) {
	system.RecordError(ctx, models.ServiceError{
		ServiceName:       constants.ServiceNormalizer,
		ErrorName:         errName,
		Error:             normErr.Error(),
		ProjectID:         project.ID,
		Platform:          project.Platform,
		PlatformUpdatedAt: project.PlatformUpdatedAt,
		RawLLMContent:     rawContent,
	})
}
