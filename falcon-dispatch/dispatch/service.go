package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	environment "hblabs.co/falcon/common/environment"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/modules/embeddings"
	"hblabs.co/falcon/modules/qdrant"
)

const (
	defaultTopN           = 20
	defaultScoreThreshold = float32(0.75)
	// cv.indexed backfill defaults. A newly-indexed CV needs matches
	// computed against recent projects so the user doesn't open the app
	// to an empty list. 48h covers a weekend's worth of scrapes; the
	// cap guards against a pathological case (scraper just ran a huge
	// catchup) embedding hundreds of projects in one blocking handler.
	defaultBackfillHours       = 48
	defaultBackfillMaxProjects = 500
)

// Service consumes project events, searches for matching CVs in Qdrant,
// and publishes a match.pending message for each candidate above the threshold.
type Service struct {
	embeddings         *embeddings.Client
	qdrant             *qdrant.Client
	topN               int
	threshold          float32
	backfillHours      int
	backfillMaxProject int
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
		embeddings:         emb,
		qdrant:             qdr,
		topN:               environment.ParseInt("DISPATCH_TOP_N", defaultTopN),
		threshold:          environment.ParseFloat32("DISPATCH_SCORE_THRESHOLD", defaultScoreThreshold),
		backfillHours:      environment.ParseInt("DISPATCH_CV_BACKFILL_HOURS", defaultBackfillHours),
		backfillMaxProject: environment.ParseInt("DISPATCH_CV_BACKFILL_MAX_PROJECTS", defaultBackfillMaxProjects),
	}, nil
}

// Run subscribes to project events (forward dispatch) and cv.indexed
// (reverse dispatch / backfill) and blocks until ctx is cancelled.
func (s *Service) Run() error {
	ctx := system.Ctx()

	for _, subject := range []string{constants.SubjectProjectCreated, constants.SubjectProjectUpdated} {
		consumer := "dispatch-" + strings.ReplaceAll(subject, ".", "-")
		if err := system.Subscribe(ctx, constants.StreamProjects, consumer, subject, s.handleProjectEvent); err != nil {
			return fmt.Errorf("subscribe %s: %w", subject, err)
		}
		logrus.Infof("subscribed to %s", subject)
	}

	// Reverse dispatch: cv.indexed fires when a new CV (or an update)
	// is ready in Qdrant. We backfill matches against the last N hours
	// of projects so the user doesn't open the app to an empty list.
	if err := system.Subscribe(ctx, constants.StreamStorage, "dispatch-cv-indexed",
		constants.SubjectCVIndexed, s.handleCVIndexed); err != nil {
		return fmt.Errorf("subscribe %s: %w", constants.SubjectCVIndexed, err)
	}
	logrus.Infof("subscribed to %s (backfill window: %dh, cap: %d projects)",
		constants.SubjectCVIndexed, s.backfillHours, s.backfillMaxProject)

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
	vector, err := s.embeddings.Embed(ctx, text, map[string]any{
		"project_id": event.ProjectID,
		"platform":   event.Platform,
		"path":       "forward",
	})
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

// handleCVIndexed is the reverse-dispatch path: a freshly indexed CV
// gets scored against every recent project so the user lands on the
// app with pre-computed matches instead of an empty list.
//
// Shape of the work:
//  1. Pull the latest N projects whose DisplayUpdatedAt falls inside
//     the backfill window (default 48h).
//  2. Embed each project text, then query Qdrant filtered by
//     this CV's payload. We search *this CV's chunks only* — cheap
//     per-project since the filter narrows the candidate set before
//     the cosine step.
//  3. For every chunk that clears the threshold, publish a
//     match.pending with the best chunk score. match-engine picks it
//     up and runs the real LLM scoring; upsert-by-(cv_id, project_id)
//     means this can safely race against the forward dispatch for
//     the same pair without duplicating rows.
//
// Runs serially — a CV indexed once doesn't need parallelism, and
// embedding 500 projects sequentially on a local Ollama takes ~40s,
// well within a consumer ack window.
func (s *Service) handleCVIndexed(data []byte) error {
	var evt models.CVIndexedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		logrus.Errorf("unmarshal cv.indexed: %v (dropping)", err)
		return nil
	}

	log := logrus.WithFields(logrus.Fields{
		"cv_id":   evt.CVID,
		"user_id": evt.UserID,
	})
	log.Infof("cv.indexed — backfilling matches from last %dh of projects", s.backfillHours)

	ctx := context.Background()

	since := time.Now().Add(-time.Duration(s.backfillHours) * time.Hour)
	filter := bson.M{
		"display_updated_at": bson.M{"$gte": since},
		"platform":           bson.M{"$ne": "freelance.de"}, // same exclusion the API applies
	}

	var projects []models.PersistedProject
	// FindPage with page=1 + pageSize=cap gives us "top cap docs,
	// most-recent first" — equivalent to a sorted-limit query.
	if _, err := system.GetStorage().FindPage(ctx,
		constants.MongoProjectsCollection,
		filter,
		"display_updated_at", true, // desc — most recent first
		1, s.backfillMaxProject,
		&projects,
	); err != nil {
		return fmt.Errorf("list recent projects: %w", err)
	}

	if len(projects) == 0 {
		log.Info("no recent projects in window — nothing to backfill")
		return nil
	}

	log.Infof("scanning %d recent projects for matches", len(projects))

	matched := 0
	for i, p := range projects {
		text := p.Title + "\n" + p.Description
		vec, err := s.embeddings.Embed(ctx, text, map[string]any{
			"cv_id":      evt.CVID,
			"project_id": p.ID,
			"platform":   p.Platform,
			"path":       "backfill",
			"i":          fmt.Sprintf("%d/%d", i+1, len(projects)),
		})
		if err != nil {
			log.Warnf("embed project %s: %v — skipping", p.ID, err)
			continue
		}

		// Filter-by-cv_id so we only see chunks belonging to *this* CV.
		// Limit is the number of chunks a single CV might have — 20 is
		// generous (typical CV has 6–12).
		results, err := s.qdrant.SearchByPayload(ctx, vec, 20, s.threshold, "cv_id", evt.CVID)
		if err != nil {
			log.Warnf("qdrant search for project %s: %v — skipping", p.ID, err)
			continue
		}
		if len(results) == 0 {
			continue
		}

		// Best chunk across the filtered results → that's what the
		// MatchPending event carries forward.
		best := results[0]
		for _, r := range results {
			if r.Score > best.Score {
				best = r
			}
		}

		msg := best.GetEvent(p.ID, p.Platform)
		// The search was already filtered to this CV, but GetEvent pulls
		// user_id from the payload too — belt-and-suspenders, make sure
		// the message carries the right user we were told about.
		if msg.UserID == "" {
			msg.UserID = evt.UserID
		}
		if err := system.Publish(ctx, constants.SubjectMatchPending, msg); err != nil {
			log.Errorf("publish match.pending for project %s: %v", p.ID, err)
			continue
		}
		matched++
	}

	log.Infof("cv.indexed backfill done — %d/%d projects matched (threshold %.2f)",
		matched, len(projects), s.threshold)
	return nil
}
