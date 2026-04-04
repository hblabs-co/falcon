package freelancede

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
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
		toFetch, err := collectNewCandidates(system.Ctx())
		if err != nil {
			getLogger().Errorf("collect candidates: %v", err)
			return
		}

		total := len(toFetch)
		if total == 0 {
			getLogger().Info("no new or updated projects")
			return
		}

		helpers.Reverse(&toFetch)
		for i, c := range toFetch {
			c.Total = total
			c.Current = i + 1
		}

		getLogger().Infof("%d projects to fetch", total)
		system.BatchProcess(system.Ctx(), toFetch, system.BatchCfg(), processOneCandidate)
	})
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
	// the comparison stays consistent across restarts.
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

	if err := system.Publish(ctx, subject, p.GetEvent()); err != nil {
		inspector.GetLogger().Errorf("publish %s: %v", subject, err)
	} else {
		inspector.GetLogger().Infof("published %s for %s", subject, p.GetId())
	}
}
