package project

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
	"hblabs.co/falcon/modules/llm"
)

// NormalizedProject is the document stored in projects_normalized.
type NormalizedProject struct {
	ProjectID        string         `json:"project_id"         bson:"project_id"`
	CompanyID        string         `json:"company_id"         bson:"company_id"`
	DisplayUpdatedAt time.Time      `json:"display_updated_at" bson:"display_updated_at"`
	En               map[string]any `json:"en"                 bson:"en"`
	De               map[string]any `json:"de"                 bson:"de"`
	Es               map[string]any `json:"es"                 bson:"es"`
	NormalizedAt     time.Time      `json:"normalized_at"      bson:"normalized_at"`
	LLMModel         string         `json:"llm_model"          bson:"llm_model"`
	LLMProvider      string         `json:"llm_provider"       bson:"llm_provider"`
}

// Service normalizes projects via LLM.
type Service struct {
	llm             *llm.Client
	normalizePrompt string
}

var indexes = []system.StorageIndexSpec{
	system.NewIndexSpec(constants.MongoNormalizedProjectsCollection, "project_id", true),
	system.NewIndexSpec(constants.MongoNormalizedProjectsCollection, "company_id", false),
	system.NewIndexSpec(constants.MongoNormalizedProjectsCollection, "display_updated_at", false),
}

// NewService creates the project normalizer and ensures DB indexes.
func NewService(ctx context.Context, llmClient *llm.Client, normalizePrompt string) (*Service, error) {
	if err := system.GetStorage().EnsureIndex(ctx, indexes...); err != nil {
		return nil, fmt.Errorf("ensure indexes: %w", err)
	}
	return &Service{llm: llmClient, normalizePrompt: normalizePrompt}, nil
}

// Register subscribes to match.pending first (demand signal — a user will see this project
// soon via match.result), then project events, then starts the retry worker.
func (s *Service) Register(ctx context.Context) error {
	if err := system.Subscribe(ctx, constants.StreamMatches, "normalizer-match-priority", constants.SubjectMatchPending, s.handleMatchPending); err != nil {
		return fmt.Errorf("subscribe match.pending: %w", err)
	}
	if err := system.Subscribe(ctx, constants.StreamProjects, "normalizer-created", constants.SubjectProjectCreated, s.handleEvent); err != nil {
		return fmt.Errorf("subscribe project.created: %w", err)
	}
	if err := system.Subscribe(ctx, constants.StreamProjects, "normalizer-updated", constants.SubjectProjectUpdated, s.handleEvent); err != nil {
		return fmt.Errorf("subscribe project.updated: %w", err)
	}
	s.startRetryWorker(ctx)
	logrus.Info("[project] subscribed to match.pending (priority), project.created, project.updated")
	return nil
}

func (s *Service) handleEvent(data []byte) error {
	var event models.ProjectEvent
	if err := json.Unmarshal(data, &event); err != nil {
		logrus.Errorf("unmarshal project event: %v (dropping)", err)
		return nil
	}
	return s.normalizeByProjectID(context.Background(), event.ProjectID)
}

func (s *Service) normalizeByProjectID(ctx context.Context, projectID string) error {
	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, projectID, &project); err != nil {
		return fmt.Errorf("fetch project %s: %w", projectID, err)
	}

	log := logrus.WithFields(logrus.Fields{
		"project_id": project.ID,
		"platform":   project.Platform,
	})

	// Idempotency: skip if we already normalized this version (or a newer one).
	// Query by project_id only, then compare dates in code — avoids subtle
	// mismatches from timestamp precision or write-lag in MongoDB.
	var existing NormalizedProject
	if err := system.GetStorage().Get(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": project.ID},
		&existing,
	); err == nil && !existing.DisplayUpdatedAt.IsZero() &&
		!existing.DisplayUpdatedAt.Before(project.DisplayUpdatedAt) {
		log.Info("project already normalized for this version, skipping")
		return nil
	}

	log.Info("normalizing project")

	projectJSON, err := json.Marshal(project)
	if err != nil {
		return fmt.Errorf("marshal project: %w", err)
	}

	userPrompt := strings.ReplaceAll(
		"Extract and normalize the following project JSON according to your instructions.\nRespond ONLY with the JSON object (no language wrapper keys). No markdown, no explanation.\n\n{{project_json}}",
		"{{project_json}}", string(projectJSON))

	deContent, rawDE, normErr := s.llm.NormalizeDE(ctx, s.normalizePrompt, userPrompt, project.ID)
	if normErr != nil {
		s.saveError(ctx, &project, constants.ErrNameNormalizerLLMParse, normErr, rawDE)
		return nil
	}
	log.Infof("step 1 done (de_keys=%d)", len(deContent))

	en, es, rawTranslate, transErr := s.llm.TranslateToEnEs(ctx, deContent, map[string]any{
		"project_id": project.ID,
		"platform":   project.Platform,
	})
	if transErr != nil {
		s.saveError(ctx, &project, constants.ErrNameNormalizerTranslateParse, transErr, rawTranslate)
		return nil
	}
	log.Infof("step 2 done (en_keys=%d es_keys=%d)", len(en), len(es))

	// The LLM is unreliable at extracting the company name (confuses it with the
	// recruiter, picks up a random string, or invents one). Override it with the
	// authoritative value from the companies collection (looked up by platform
	// company_id + source), falling back to the platform name when unknown.
	authoritativeCompany := resolveCompanyName(ctx, &project)
	if authoritativeCompany != "" {
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

	log.Infof("project normalized and published — en_keys=%d de_keys=%d es_keys=%d", len(en), len(deContent), len(es))
	return nil
}

func (s *Service) saveError(ctx context.Context, project *models.PersistedProject, errName string, normErr error, rawContent string) {
	system.RecordError(ctx, models.ServiceError{
		ServiceName:   constants.ServiceNormalizer,
		ErrorName:     errName,
		Error:         normErr.Error(),
		ProjectID:     project.ID,
		Platform:      project.Platform,
		RawLLMContent: rawContent,
	})
}

// resolveCompanyName returns the authoritative company name: the one stored in
// the companies collection (matched by platform + company_id), or the platform
// name as fallback when the company is unknown.
func resolveCompanyName(ctx context.Context, project *models.PersistedProject) string {
	if project.CompanyID != "" {
		var company models.Company
		err := system.GetStorage().Get(ctx, constants.MongoCompaniesCollection,
			bson.M{"company_id": project.CompanyID, "source": project.Platform},
			&company)
		if err == nil && company.CompanyName != "" {
			return company.CompanyName
		}
	}
	return project.Platform
}

// overrideCompanyName sets data.company.name, preserving other company fields
// (hiring_type, is_direct_client, etc.) when present.
func overrideCompanyName(data map[string]any, name string) {
	if co, ok := data["company"].(map[string]any); ok {
		co["name"] = name
		data["company"] = co
		return
	}
	data["company"] = map[string]any{"name": name}
}

