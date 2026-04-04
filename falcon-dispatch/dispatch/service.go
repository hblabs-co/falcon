package dispatch

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/embeddings"
	"hblabs.co/falcon/modules/qdrant"
)


const (
	defaultTopN           = 20
	defaultScoreThreshold = float32(0.75)
)

// Service consumes project events, searches for matching CVs in Qdrant,
// and publishes a match.pending message for each candidate above the threshold.
type Service struct {
	embeddings *embeddings.Client
	qdrant     *qdrant.Client
	topN       int
	threshold  float32
}

func NewService() (*Service, error) {
	emb, err := embeddings.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("embeddings client: %w", err)
	}

	qdr, err := qdrant.NewFromEnv()
	if err != nil {
		return nil, fmt.Errorf("qdrant client: %w", err)
	}

	return &Service{
		embeddings: emb,
		qdrant:     qdr,
		topN:       helpers.ParseInt("DISPATCH_TOP_N", defaultTopN),
		threshold:  helpers.ParseFloat32("DISPATCH_SCORE_THRESHOLD", defaultScoreThreshold),
	}, nil
}

// Run subscribes to project.created and project.updated and blocks until ctx is cancelled.
func (s *Service) Run() error {
	ctx := system.Ctx()

	for _, subject := range []string{constants.SubjectProjectCreated, constants.SubjectProjectUpdated} {
		consumer := "dispatch-" + subject
		if err := system.Subscribe(ctx, constants.StreamProjects, consumer, subject, s.handleProjectEvent); err != nil {
			return fmt.Errorf("subscribe %s: %w", subject, err)
		}
		logrus.Infof("subscribed to %s", subject)
	}

	system.Wait()
	return nil
}

func (s *Service) handleProjectEvent(data []byte) error {
	var event models.ProjectEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal project event: %w", err)
	}

	log := logrus.WithFields(logrus.Fields{
		"project_id": event.ProjectID,
		"platform":   event.Platform,
	})
	log.Infof("received project event — searching for matches")

	ctx := context.Background()

	var project models.PersistedProject
	if err := system.GetStorage().GetById(ctx, constants.MongoProjectsCollection, event.ProjectID, &project); err != nil {
		return fmt.Errorf("fetch project %s: %w", event.ProjectID, err)
	}

	text := project.Title + "\n" + project.Description
	vector, err := s.embeddings.Embed(ctx, text)
	if err != nil {
		return fmt.Errorf("embed project: %w", err)
	}

	results, err := s.qdrant.Search(ctx, vector, s.topN, s.threshold)
	if err != nil {
		return fmt.Errorf("qdrant search: %w", err)
	}
	log.Infof("qdrant returned %d candidates above threshold %.2f", len(results), s.threshold)

	for _, r := range results {
		msg := models.MatchPending{
			CVID:       r.Payload["cv_id"],
			QdrantID:   r.ID,
			UserID:     r.Payload["user_id"],
			ProjectID:  event.ProjectID,
			Platform:   event.Platform,
			Similarity: r.Score,
		}
		if err := system.Publish(ctx, constants.SubjectMatchPending, msg); err != nil {
			log.Errorf("publish match.pending for user %s: %v", msg.UserID, err)
		}
	}

	return nil
}
