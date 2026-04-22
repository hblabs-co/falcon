package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"

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
		consumer := "dispatch-" + strings.ReplaceAll(subject, ".", "-")
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

	// With multi-vector storage there are multiple points per CV (one
	// per chunk). We pull a large-ish window so we see every chunk of
	// every CV that could be relevant, then aggregate by cv_id below.
	searchLimit := s.topN * 4
	results, err := s.qdrant.Search(ctx, vector, searchLimit, s.threshold)
	if err != nil {
		return fmt.Errorf("qdrant search: %w", err)
	}

	// Group by cv_id. For each CV we track:
	//   - the best chunk (max score) → the "single strongest section"
	//   - the count of matching chunks above threshold → the "breadth"
	//     of coverage. Both signals matter: a CV that matches one
	//     chunk at 0.72 is weaker than a CV matching 9 chunks at 0.72.
	type group struct {
		best  qdrant.SearchResult
		count int
	}
	byCV := make(map[string]*group, len(results))
	for _, r := range results {
		cvID := r.Payload["cv_id"]
		if cvID == "" {
			continue
		}
		g, ok := byCV[cvID]
		if !ok {
			g = &group{best: r}
			byCV[cvID] = g
		} else if r.Score > g.best.Score {
			g.best = r
		}
		g.count++
	}

	// Composite score = max_score * (1 + ln(1 + chunk_count)).
	// Rewards CVs with many matching sections while still respecting
	// the raw similarity of their best section. See chat notes:
	//   9 chunks @ 0.721 → 2.38   (strong broad + specific match)
	//   1 chunk  @ 0.605 → 1.02   (narrow single-section match)
	type ranked struct {
		cvID      string
		best      qdrant.SearchResult
		count     int
		composite float32
	}
	list := make([]ranked, 0, len(byCV))
	for cvID, g := range byCV {
		composite := g.best.Score * float32(1+math.Log(1+float64(g.count)))
		list = append(list, ranked{
			cvID:      cvID,
			best:      g.best,
			count:     g.count,
			composite: composite,
		})
	}

	// Sort by composite desc, take top N. Ties broken by raw max score.
	sort.Slice(list, func(i, j int) bool {
		if list[i].composite != list[j].composite {
			return list[i].composite > list[j].composite
		}
		return list[i].best.Score > list[j].best.Score
	})
	if len(list) > s.topN {
		list = list[:s.topN]
	}

	// Log what's going out — max score, chunk count, composite. Gives
	// us visibility into why a CV won or lost the ranking.
	dump := make([]string, 0, len(list))
	for _, r := range list {
		dump = append(dump, fmt.Sprintf("%s=%.3f×%d→%.2f",
			r.cvID, r.best.Score, r.count, r.composite))
	}
	log.Infof("qdrant returned %d chunks → %d unique CVs → top %d by composite — [%s]",
		len(results), len(byCV), len(list), strings.Join(dump, ", "))

	for _, r := range list {
		msg := r.best.GetEvent(event.ProjectID, event.Platform)
		if err := system.Publish(ctx, constants.SubjectMatchPending, msg); err != nil {
			log.Errorf("publish match.pending for user %s: %v", msg.UserID, err)
		}
	}

	return nil
}
