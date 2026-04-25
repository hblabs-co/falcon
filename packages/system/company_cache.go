package system

import (
	"context"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
)

// companyCacheTTL balances freshness vs Mongo round-trip savings.
// Company records change rarely (new logo, occasional rename); a
// stale name or logo on a downstream surface is cosmetic, not
// incorrect, so 1h is conservative enough that backfills get picked
// up the same working day while still eliminating ~100x of lookups
// when the same company appears across many events.
const companyCacheTTL = 1 * time.Hour

type companyCacheEntry struct {
	company models.Company
	found   bool
	expires time.Time
}

var (
	companyCacheMu      sync.RWMutex
	companyCacheEntries = map[string]companyCacheEntry{}
)

// GetCachedCompany returns the full Company document referenced by
// the given project, read from a process-wide TTL cache with a Mongo
// fallback on miss. The second return value reports whether the
// company was found; when false, the returned Company is zero-valued.
// Callers decide which fields they care about (name, logo URL,
// recruiter stats, metadata, etc.).
//
// Misses are cached too so unknown company_ids don't keep rehitting
// Mongo within the TTL window. Per-replica (no cross-pod sync); the
// worst case for staleness is a user seeing "old" cached data for up
// to the TTL after a backfill.
func GetCachedCompany(ctx context.Context, project *models.PersistedProject) (models.Company, bool) {
	if project == nil || project.CompanyID == "" {
		return models.Company{}, false
	}
	companyID, platform := project.CompanyID, project.Platform
	key := platform + ":" + companyID

	companyCacheMu.RLock()
	e, ok := companyCacheEntries[key]
	companyCacheMu.RUnlock()
	if ok && time.Now().Before(e.expires) {
		return e.company, e.found
	}

	var company models.Company
	expires := time.Now().Add(companyCacheTTL)
	if err := GetStorage().Get(ctx, constants.MongoCompaniesCollection,
		bson.M{"company_id": companyID, "source": platform},
		&company); err != nil {
		companyCacheMu.Lock()
		companyCacheEntries[key] = companyCacheEntry{expires: expires}
		companyCacheMu.Unlock()
		return models.Company{}, false
	}

	companyCacheMu.Lock()
	companyCacheEntries[key] = companyCacheEntry{
		company: company,
		found:   true,
		expires: expires,
	}
	companyCacheMu.Unlock()
	return company, true
}
