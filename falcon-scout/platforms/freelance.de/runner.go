package freelancede

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

func getLogger() *logrus.Entry {
	return logrus.WithFields(logrus.Fields{"source": Source})
}

var indexes = []system.StorageIndexSpec{
	system.NewIndexSpec(constants.MongoProjectsCollection, "platform_id", true),
	system.NewIndexSpec(constants.MongoScrapeFailuresCollection, "platform_id", false),
}

func Run() {
	getLogger().Infof("starting — polling every %s (Ctrl+C to stop)", system.PollInterval())

	for _, spec := range indexes {
		if err := system.GetStorage().EnsureIndex(system.Ctx(), spec); err != nil {
			getLogger().Errorf("ensure index %s.%s: %v", spec.Collection, spec.Field, err)
		}
	}

	if err := getSession().Login(); err != nil {
		getLogger().Fatalf("login failed: %v", err)
	}

	system.Poll(system.Ctx(), system.PollInterval(), getLogger(), func() {
		// TODO: ScrapeProjectCandidates does not paginate — only returns the first page.
		// candidates, err := ScrapeProjectCandidates(system.Ctx())
		candidates, err := FetchProjectCandidates(system.Ctx())
		if err != nil {
			getLogger().Errorf("fetch project candidates: %v", err)
			return
		}
		candidatesLenght := len(candidates)
		if candidatesLenght == 0 {
			getLogger().Warn("no project candidates found — may indicate a scraping or fetch issue")
			return
		}
		getLogger().Infof("found %d project candidates", candidatesLenght)
		processManyCandidates(system.Ctx(), candidates)
	})
}

// processCandidates compares scraped candidates against stored projects and
// fetches full details for those that are new or updated.
func processManyCandidates(ctx context.Context, candidates []*ProjectCandidate) {
	// Extract platform IDs to query only the projects we need to compare.
	platformIDs := make([]string, len(candidates))
	for i, c := range candidates {
		platformIDs[i] = c.PlatformID
	}

	// Fetch existing persisted projects for these platform IDs.
	var existing []models.PersistedProject
	if err := system.GetStorage().GetManyByField(ctx, constants.MongoProjectsCollection, "platform_id", platformIDs, &existing); err != nil {
		getLogger().Errorf("fetch existing projects: %v", err)
	}

	existingMap := make(map[string]models.PersistedProject, len(existing))
	for _, p := range existing {
		existingMap[p.PlatformID] = p
	}
	getLogger().Debugf("DEBUG: found %d existing projects in db out of %d candidates", len(existing), len(candidates))

	// Determine which candidates need a full detail fetch.
	var toFetch []*ProjectCandidate
	for _, c := range candidates {
		e, found := existingMap[c.PlatformID]
		if !found {
			getLogger().Debugf("DEBUG: %s not found in db", c.PlatformID)
		} else if e.PlatformUpdatedAt != c.PlatformUpdatedAt {
			getLogger().Debugf("DEBUG: %s updated — stored=%q candidate=%q", c.PlatformID, e.PlatformUpdatedAt, c.PlatformUpdatedAt)
		}
		if !found || e.PlatformUpdatedAt != c.PlatformUpdatedAt {
			c.ExistingID = e.ID
			toFetch = append(toFetch, c)
		}
	}

	total := len(toFetch)
	for i, j := 0, total-1; i < j; i, j = i+1, j-1 {
		toFetch[i], toFetch[j] = toFetch[j], toFetch[i]
	}
	for index, c := range toFetch {
		c.Total = total
		c.Current = index + 1
	}

	getLogger().Infof("%d projects need detail fetch (new or updated)", total)
	system.BatchProcess(ctx, toFetch, system.BatchCfg(), processOneCandidate)
}

// processOneCandidate fetches full project details for a candidate and upserts a PersistedProject to MongoDB.
func processOneCandidate(ctx context.Context, c *ProjectCandidate) {
	inspector := &Inspector{Url: c.URL, PlatformID: c.PlatformID, Current: c.Current, Total: c.Total}

	result, err := inspector.Inspect()
	if err != nil {
		inspector.GetLogger().Errorf("inspect failed: %v", err)
		inspector.SaveFailure(ctx, err)
		return
	}

	p := models.NewPersistedProject(result, c.PlatformID, Source, time.Now(), c.ExistingID)
	// Use the candidate's PlatformUpdatedAt (ISO 8601 from the API) rather than
	// the scraped detail value, which may be in a different format. This ensures
	// the comparison in processManyCandidates stays consistent across restarts.
	p.PlatformUpdatedAt = c.PlatformUpdatedAt
	if err := system.GetStorage().Replace(ctx, constants.MongoProjectsCollection, p); err != nil {
		inspector.GetLogger().Errorf("replace project %s: %v", p.PlatformID, err)
		return
	}
	inspector.GetLogger().Infof("project saved with internal id %s", p.GetId())

	subject := constants.SubjectProjectCreated
	if c.ExistingID != "" {
		subject = constants.SubjectProjectUpdated
	}

	event := p.GetEvent()
	if err := system.Publish(ctx, subject, event); err != nil {
		inspector.GetLogger().Errorf("publish %s: %v", subject, err)
	} else {
		inspector.GetLogger().Infof("published %s for %s", subject, p.GetId())
	}
}
