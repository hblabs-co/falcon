package match

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/llm"
)

const defaultScoreThreshold = float32(6.0)

// Service consumes match.pending events, calls the LLM to score each CV/project
// pair (in German, then translated to EN+ES), and publishes match.result for
// candidates above the score threshold.
type Service struct {
	scorer    *scorer
	threshold float32
}

var indexes = []system.CompoundIndexSpec{
	{
		Collection: constants.MongoMatchResultsCollection,
		Fields:     []string{"cv_id", "project_id"},
		Unique:     true,
	},
	{
		Collection: constants.MongoMatchResultsCollection,
		Fields:     []string{"user_id", "scored_at"},
		Unique:     false,
	},
}

func NewService(ctx context.Context) (*Service, error) {
	llmClient, err := llm.NewFromEnv("") // translate prompt provided per-call in scorer
	if err != nil {
		return nil, fmt.Errorf("llm client: %w", err)
	}

	for _, idx := range indexes {
		if err := system.GetStorage().EnsureCompoundIndex(ctx, idx); err != nil {
			return nil, fmt.Errorf("ensure index %v: %w", idx.Fields, err)
		}
	}

	return &Service{
		scorer:    newScorer(llmClient),
		threshold: helpers.ParseFloat32("MATCH_ENGINE_SCORE_THRESHOLD", defaultScoreThreshold),
	}, nil
}

// Run subscribes to match.pending and blocks until ctx is cancelled.
// Scale by adding replicas — all pods share the durable consumer "match-engine"
// so NATS delivers each message to exactly one pod.
func (s *Service) Run() error {
	ctx := system.Ctx()

	if err := system.Subscribe(ctx, constants.StreamMatches, "match-engine", constants.SubjectMatchPending, s.handleMatchPending); err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	logrus.Infof("subscribed to %s", constants.SubjectMatchPending)

	// Flip match_results.normalized=true when normalizer finishes a
	// project. Event-driven path — fastest reaction to normalization.
	if err := system.Subscribe(ctx, constants.StreamProjects, "match-engine-normalized",
		constants.SubjectProjectNormalized, s.handleProjectNormalized); err != nil {
		return fmt.Errorf("subscribe project.normalized: %w", err)
	}
	logrus.Infof("subscribed to %s", constants.SubjectProjectNormalized)

	s.startRetryWorker(ctx)
	// Safety net for the event-driven path: if the project.normalized
	// event was missed (pod restart, consumer drift) this sweep fixes
	// stale flags on its own schedule.
	s.startNormalizedSweep(ctx)

	system.Wait()
	return nil
}

func (s *Service) handleMatchPending(data []byte) error {
	var event models.MatchPendingEvent
	if err := json.Unmarshal(data, &event); err != nil {
		logrus.Errorf("unmarshal match.pending: %v (dropping)", err)
		return nil
	}

	log := logrus.WithFields(logrus.Fields{
		"cv_id":      event.CVID,
		"project_id": event.ProjectID,
		"user_id":    event.UserID,
	})

	ctx := context.Background()

	// Fetch CV. If missing (deleted or race), drop the event — retrying won't help.
	var cv models.PersistedCV
	if err := system.GetStorage().GetById(ctx, constants.MongoCVsCollection, event.CVID, &cv); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Info("cv not found — dropping match.pending")
			return nil
		}
		return fmt.Errorf("fetch cv: %w", err)
	}
	if cv.ExtractedText == "" {
		log.Warn("cv has no extracted text — dropping match.pending")
		return nil
	}

	// Fetch project. Same treatment for missing.
	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, event.ProjectID, &project); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Info("project not found — dropping match.pending")
			return nil
		}
		return fmt.Errorf("fetch project: %w", err)
	}

	// Prefer the normalizer's cleaned display title (no "Projekt-Nr:
	// 62737 -" prefixes, no "(m/w/d)") if it's already available.
	// match-engine can race ahead of the normalizer — fall back to the
	// raw scraped title in that case.
	displayTitle := cleanProjectTitle(ctx, event.ProjectID, project.Title)

	log.Info("scoring CV/project pair")

	result, rawContent, err := s.scorer.Score(ctx, displayTitle, project.Description, cv.ExtractedText, logrus.Fields{
		"cv_id":      event.CVID,
		"project_id": event.ProjectID,
	})
	if err != nil {
		recordScoreError(ctx, event, err, rawContent)
		log.Warnf("score failed: %v (recorded for retry)", err)
		return nil
	}

	// Authoritative company fields from the companies collection (LLM
	// name extraction is unreliable; logo only exists in Mongo). Goes
	// through the process-wide cache so a burst of match.pending for
	// the same company only hits Mongo once per TTL.
	company, _ := system.GetCachedCompany(ctx, &project)

	result.CVID = event.CVID
	result.UserID = event.UserID
	result.ProjectID = event.ProjectID
	// Title fallback chain:
	//   1. normalized display      (projects_normalized.<lang>.title.display)
	//   2. LLM-cleaned title       (result.ProjectTitle set by scorer.Score)
	//   3. raw scraped title       (last resort)
	// cleanProjectTitle returns (1) or falls back to raw. When it fell
	// back (normalizer lost the race), prefer the LLM-cleaned value that
	// came out of this same scoring pass.
	if displayTitle == project.Title && result.ProjectTitle != "" {
		// keep LLM-cleaned result.ProjectTitle
	} else {
		result.ProjectTitle = displayTitle
	}
	result.Platform = event.Platform
	result.CompanyName = company.CompanyName
	result.CompanyLogoURL = company.LogoMinioURL
	result.PassedThreshold = result.Score >= s.threshold
	result.ScoredAt = time.Now()
	// The normalizer can still be working when match-engine already has
	// enough to score (title + description come from the raw doc). Mark
	// the match unnormalized if projects_normalized.<project_id> is
	// missing; the event handler or periodic sweep flips it later.
	result.Normalized = isProjectNormalized(ctx, event.ProjectID)

	// Upsert by (cv_id, project_id) so re-scoring (e.g. project updated) overwrites
	// the previous result instead of creating duplicates. Unique compound index on
	// (cv_id, project_id) enforces this at the DB layer.
	filter := bson.M{"cv_id": event.CVID, "project_id": event.ProjectID}
	if err := system.GetStorage().Set(ctx, constants.MongoMatchResultsCollection, filter, result); err != nil {
		log.Errorf("save match result: %v", err)
		return nil
	}

	if !result.PassedThreshold {
		log.Infof("score %.1f below threshold %.1f — saved, not forwarded", result.Score, s.threshold)
		return nil
	}

	if err := system.Publish(ctx, constants.SubjectMatchResult, result); err != nil {
		log.Errorf("publish match.result: %v", err)
		return nil
	}

	log.Infof("published match.result — score %.1f label=%s", result.Score, result.Label)
	return nil
}

// resolveCompanyName returns the authoritative company name for the project,
// or empty if unknown. Matches the pattern used by falcon-normalizer.
// cleanProjectTitle returns the normalized display title from
// projects_normalized.<lang>.title.display when it exists, falling
// back to rawTitle otherwise. "de" is preferred because the original
// German stays closest to what the scraper captured; if missing we
// step through en/es. Called by the match-engine so the LLM prompt
// and the stored match_result both use the human-readable title
// rather than the raw scrape (which often carries platform job-
// number prefixes like "Projekt-Nr: 62737 -").
func cleanProjectTitle(ctx context.Context, projectID, rawTitle string) string {
	var doc struct {
		De map[string]any `bson:"de"`
		En map[string]any `bson:"en"`
		Es map[string]any `bson:"es"`
	}
	if err := system.GetStorage().Get(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": projectID}, &doc); err != nil {
		return rawTitle
	}
	for _, lang := range []map[string]any{doc.De, doc.En, doc.Es} {
		if lang == nil {
			continue
		}
		title, _ := lang["title"].(map[string]any)
		if title == nil {
			continue
		}
		if display, ok := title["display"].(string); ok && display != "" {
			return display
		}
	}
	return rawTitle
}


func recordScoreError(ctx context.Context, event models.MatchPendingEvent, err error, rawContent string) {
	system.RecordError(ctx, models.ServiceError{
		ServiceName:   constants.ServiceMatchEngine,
		ErrorName:     constants.ErrNameMatchLLMFailed,
		Error:         err.Error(),
		ProjectID:     event.ProjectID,
		CVID:          event.CVID,
		UserID:        event.UserID,
		Platform:      event.Platform,
		RawLLMContent: rawContent,
	})
}
