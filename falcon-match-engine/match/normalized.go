package match

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// normalizedSweepInterval is how often the safety-net loop looks for
// match_results still flagged normalized=false. One minute is frequent
// enough that the UI spinner clears quickly in the worst case, rare
// enough that the DB load stays negligible.
const normalizedSweepInterval = 1 * time.Minute

// isProjectNormalized returns true when projects_normalized already has
// the document for projectID — i.e. iOS can safely fetch /projects/:id
// without hitting 404. Called when saving a match_result to decide the
// initial value of the normalized flag.
func isProjectNormalized(ctx context.Context, projectID string) bool {
	n, err := system.GetStorage().Count(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": projectID})
	if err != nil {
		// Treat DB errors as "not normalized" so we err on the side of
		// showing a spinner in the client instead of a broken sheet.
		logrus.Warnf("isProjectNormalized(%s): %v — defaulting to false", projectID, err)
		return false
	}
	return n > 0
}

// handleProjectNormalized flips match_results.normalized=true for every
// match whose project just finished normalization. Runs once per
// project.normalized event — low volume, cheap.
func (s *Service) handleProjectNormalized(data []byte) error {
	var evt models.ProjectNormalizedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		logrus.Errorf("unmarshal project.normalized: %v (dropping)", err)
		return nil
	}
	if evt.ProjectID == "" {
		return nil
	}

	ctx := context.Background()
	modified, err := system.GetStorage().BulkUpdate(ctx, constants.MongoMatchResultsCollection,
		bson.M{"project_id": evt.ProjectID, "normalized": false},
		bson.M{"$set": bson.M{"normalized": true}})
	if err != nil {
		return err
	}
	if modified > 0 {
		logrus.Infof("project.normalized: flipped %d match_results for project %s", modified, evt.ProjectID)
	}
	return nil
}

// startNormalizedSweep runs a background loop that catches match_results
// missed by the event-driven path (pod restart right after save, NATS
// lag, consumer drift). For each match with normalized=false, it checks
// projects_normalized and flips the flag if the doc now exists.
//
// Batches by distinct project_id so N matches for the same project
// cost ONE Count() call, not N.
func (s *Service) startNormalizedSweep(ctx context.Context) {
	system.StartWorker(ctx, normalizedSweepInterval, s.runNormalizedSweep)
}

func (s *Service) runNormalizedSweep(ctx context.Context) {
	var unmarked []struct {
		ProjectID string `bson:"project_id"`
	}
	if err := system.GetStorage().GetMany(ctx, constants.MongoMatchResultsCollection,
		bson.M{"normalized": false}, &unmarked); err != nil {
		logrus.Warnf("normalized sweep: list failed: %v", err)
		return
	}
	if len(unmarked) == 0 {
		return
	}

	// Dedupe project_ids so we only call Count once per distinct project.
	unique := make(map[string]struct{}, len(unmarked))
	for _, m := range unmarked {
		if m.ProjectID != "" {
			unique[m.ProjectID] = struct{}{}
		}
	}

	var flippedProjects, flippedMatches int
	for projectID := range unique {
		if !isProjectNormalized(ctx, projectID) {
			continue
		}
		modified, err := system.GetStorage().BulkUpdate(ctx, constants.MongoMatchResultsCollection,
			bson.M{"project_id": projectID, "normalized": false},
			bson.M{"$set": bson.M{"normalized": true}})
		if err != nil {
			logrus.Warnf("normalized sweep: update for %s failed: %v", projectID, err)
			continue
		}
		if modified > 0 {
			flippedProjects++
			flippedMatches += int(modified)
		}
	}
	if flippedMatches > 0 {
		logrus.Infof("normalized sweep: flipped %d matches across %d projects", flippedMatches, flippedProjects)
	}
}
