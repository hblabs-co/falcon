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

	s.startRetryWorker(ctx)

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

	log.Info("scoring CV/project pair")

	result, rawContent, err := s.scorer.Score(ctx, project.Title, project.Description, cv.ExtractedText, logrus.Fields{
		"cv_id":      event.CVID,
		"project_id": event.ProjectID,
	})
	if err != nil {
		recordScoreError(ctx, event, err, rawContent)
		log.Warnf("score failed: %v (recorded for retry)", err)
		return nil
	}

	// Authoritative company name from companies collection (LLM unreliable here).
	companyName := resolveCompanyName(ctx, &project)

	result.CVID = event.CVID
	result.UserID = event.UserID
	result.ProjectID = event.ProjectID
	result.ProjectTitle = project.Title
	result.Platform = event.Platform
	result.CompanyName = companyName
	result.PassedThreshold = result.Score >= s.threshold
	result.ScoredAt = time.Now()

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
func resolveCompanyName(ctx context.Context, project *models.PersistedProject) string {
	if project.CompanyID == "" {
		return ""
	}
	var company models.Company
	err := system.GetStorage().Get(ctx, constants.MongoCompaniesCollection,
		bson.M{"company_id": project.CompanyID, "source": project.Platform},
		&company)
	if err != nil {
		return ""
	}
	return company.CompanyName
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
