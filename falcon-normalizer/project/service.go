package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/llm"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// NormalizedProject is the document stored in projects_normalized.
//
// Status/AcquiredAt implement an atomic "claim" pattern so the normalize
// pipeline is multi-pod safe: the first pod that wins the FindOneAnd-
// Update race calls the LLM, every other pod (and every concurrent
// goroutine within the same pod) sees status=in_progress and skips.
// Legacy docs written before this field existed are treated as
// implicitly "done" — they have display_updated_at set, which is what
// the version comparison hinges on.
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
	// Status + AcquiredAt are set during the claim and cleared once the
	// final content is written (status flips to "done"). A pod that
	// crashes mid-normalize leaves status="in_progress"; the TTL-style
	// check in the claim filter re-acquires it after lockTimeout.
	Status     string    `json:"status,omitempty"      bson:"status,omitempty"`
	AcquiredAt time.Time `json:"acquired_at,omitempty" bson:"acquired_at,omitzero"`
}

// Lock lifecycle constants. lockTimeout is how long we trust an
// in_progress claim before assuming the owner died — 5 minutes is
// generous enough for slow LLM calls, short enough that a crash
// doesn't leave a project stuck for long.
const (
	statusInProgress = "in_progress"
	statusDone       = "done"
	lockTimeout      = 5 * time.Minute
)

// Service normalizes projects via LLM.
type Service struct {
	llm             *llm.Client
	normalizePrompt string
}

// NewService creates the project normalizer. Mongo indexes are
// owned by falcon-config (run as a Job pre-deploy).
func NewService(_ context.Context, llmClient *llm.Client, normalizePrompt string) (*Service, error) {
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

// normalizeByProjectID fetches the project, tries to atomically claim
// it, and — if it wins the claim — runs the LLM pipeline. If another
// pod/goroutine already holds the claim (or already produced a doc at
// the current version), this returns nil without calling the LLM.
func (s *Service) normalizeByProjectID(ctx context.Context, projectID string) error {
	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, projectID, &project); err != nil {
		// Project no longer exists (manually deleted, or the event was
		// queued before the doc was saved and then rolled back). Drop
		// the message — returning an error here would NAK and NATS
		// would redeliver forever.
		if errors.Is(err, mongo.ErrNoDocuments) {
			logrus.WithField("project_id", projectID).Info("project not found — dropping event")
			return nil
		}
		return fmt.Errorf("fetch project %s: %w", projectID, err)
	}

	log := logrus.WithFields(logrus.Fields{
		"project_id": project.ID,
		"platform":   project.Platform,
	})

	// Atomic claim: only one pod/goroutine in the cluster proceeds past
	// this point for a given (project_id, display_updated_at). Second
	// callers (whether from match.pending, project.created, or a
	// concurrent replica) see the in_progress lock and skip cleanly —
	// no duplicate LLM spend, no duplicate project.normalized push.
	claimed, err := s.tryClaim(ctx, project.ID, project.DisplayUpdatedAt)
	if err != nil {
		return fmt.Errorf("claim project %s: %w", project.ID, err)
	}
	if !claimed {
		log.Info("another worker holds the claim or this version is already normalized, skipping")
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
		s.releaseClaim(ctx, project.ID)
		return nil
	}
	log.Infof("step 1 done (de_keys=%d)", len(deContent))

	en, es, rawTranslate, transErr := s.llm.TranslateToEnEs(ctx, deContent, map[string]any{
		"project_id": project.ID,
		"platform":   project.Platform,
	})
	if transErr != nil {
		s.saveError(ctx, &project, constants.ErrNameNormalizerTranslateParse, transErr, rawTranslate)
		s.releaseClaim(ctx, project.ID)
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
		// Flip the lock to "done" — the final Set below overwrites the
		// in_progress placeholder from tryClaim with the real content.
		Status: statusDone,
	}

	if err := system.GetStorage().Set(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": project.ID}, doc); err != nil {
		// Release the claim so the NATS redelivery can retry cleanly
		// without waiting for the 5-min TTL — otherwise this pod would
		// block itself for almost the full lockTimeout before noticing
		// the lock as "stale" and then re-running the (already paid) LLM.
		s.releaseClaim(ctx, project.ID)
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

// tryClaim attempts to atomically mark this project as "being normalized
// by us". Returns true iff the caller owns the work after this call.
//
// Sequence:
//  1. First try: InsertOne a fresh in_progress doc. Succeeds iff no doc
//     for this project_id exists yet (unique index enforces exclusivity).
//  2. Duplicate-key path: a doc already exists. Run an atomic UpdateOne
//     that only matches if the existing doc is "reclaimable":
//     - stale version (display_updated_at < target), OR
//     - abandoned lock (status=in_progress with expired acquired_at).
//     If the update modifies 1 row, we took over. If 0, someone else
//     is actively working or has already produced a current doc.
//
// Writing the real content later is done via Set(), which overwrites
// this placeholder with the final doc carrying Status=done.
func (s *Service) tryClaim(ctx context.Context, projectID string, targetVersion time.Time) (bool, error) {
	now := time.Now()
	// Include DisplayUpdatedAt in the placeholder so the "stale version"
	// branch of the takeover filter can't mis-match. Without this, a
	// concurrent second goroutine would see a placeholder missing the
	// field and — because Mongo treats a missing field as null for $lt
	// comparisons (null sorts before any Date) — the filter
	// `{display_updated_at: {$lt: targetVersion}}` would match and let
	// the second caller steal the lock mid-LLM. That was the exact
	// log pattern "normalizing project" appearing twice one second
	// apart for the same project_id.
	placeholder := &NormalizedProject{
		ProjectID:        projectID,
		Status:           statusInProgress,
		AcquiredAt:       now,
		DisplayUpdatedAt: targetVersion,
	}

	err := system.GetStorage().Insert(ctx, constants.MongoNormalizedProjectsCollection, placeholder)
	if err == nil {
		return true, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return false, err
	}

	// Doc already exists — try to atomically take it over if stale or abandoned.
	filter := bson.M{
		"project_id": projectID,
		"$or": []bson.M{
			{"display_updated_at": bson.M{"$lt": targetVersion}},
			{
				"status":      statusInProgress,
				"acquired_at": bson.M{"$lt": now.Add(-lockTimeout)},
			},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"status":             statusInProgress,
			"acquired_at":        now,
			"display_updated_at": targetVersion,
		},
	}
	modified, err := system.GetStorage().UpdateOne(ctx, constants.MongoNormalizedProjectsCollection, filter, update)
	if err != nil {
		return false, err
	}
	return modified > 0, nil
}

// releaseClaim drops the in_progress placeholder so the error handler's
// retry worker (or the next natural event) can re-acquire and retry.
// Only deletes when the doc is still in the in_progress state we wrote
// — we never want to wipe a completed (status=done) doc here.
func (s *Service) releaseClaim(ctx context.Context, projectID string) {
	filter := bson.M{
		"project_id": projectID,
		"status":     statusInProgress,
	}
	if err := system.GetStorage().DeleteMany(ctx, constants.MongoNormalizedProjectsCollection, filter); err != nil {
		logrus.WithField("project_id", projectID).Warnf("release claim: %v", err)
	}
}

// resolveCompanyName returns the authoritative company name: the one
// stored in the companies collection (matched by platform + company_id
// via the process-wide cache in common/system), or the platform name
// as fallback when the company is unknown or has no name.
func resolveCompanyName(ctx context.Context, project *models.PersistedProject) string {
	if company, ok := system.GetCachedCompany(ctx, project); ok && company.CompanyName != "" {
		return company.CompanyName
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
