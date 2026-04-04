package match

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

const defaultScoreThreshold = float32(6.0)

// Service consumes match.pending events, calls the LLM to score each CV/project
// pair, and publishes match.result for candidates above the score threshold.
type Service struct {
	llm       *llmClient
	threshold float32
}

func NewService() (*Service, error) {
	llm, err := newLLMClient()
	if err != nil {
		return nil, fmt.Errorf("llm client: %w", err)
	}

	return &Service{
		llm:       llm,
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

	system.Wait()
	return nil
}

func (s *Service) handleMatchPending(data []byte) error {
	var event models.MatchPendingEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal match.pending: %w", err)
	}

	log := logrus.WithFields(logrus.Fields{
		"cv_id":      event.CVID,
		"project_id": event.ProjectID,
	})
	log.Info("scoring CV/project pair")

	ctx := context.Background()

	var cv models.PersistedCV
	if err := system.GetStorage().GetById(ctx, constants.MongoCVsCollection, event.CVID, &cv); err != nil {
		return fmt.Errorf("fetch cv %s: %w", event.CVID, err)
	}
	if cv.ExtractedText == "" {
		return fmt.Errorf("cv %s has no extracted text", event.CVID)
	}

	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, event.ProjectID, &project); err != nil {
		return fmt.Errorf("fetch project %s: %w", event.ProjectID, err)
	}

	result, err := s.llm.Score(ctx, project.Title, project.Description, cv.ExtractedText)
	if err != nil {
		return fmt.Errorf("llm score: %w", err)
	}

	if result.Score < s.threshold {
		log.Infof("score %.1f below threshold %.1f — skipping", result.Score, s.threshold)
		return nil
	}

	result.CVID = event.CVID
	result.UserID = event.UserID
	result.ProjectID = event.ProjectID
	result.Platform = event.Platform

	if err := system.Publish(ctx, constants.SubjectMatchResult, result); err != nil {
		return fmt.Errorf("publish match.result: %w", err)
	}

	log.Infof("published match.result — score %.1f label=%s", result.Score, result.Label)
	return nil
}
