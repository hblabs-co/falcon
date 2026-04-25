package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// MongoDB indexing — design notes
// ===============================
//
// Why a centralised list at all
// -----------------------------
// Every Falcon service used to ensure its own indexes at startup.
// That works (EnsureIndex is idempotent) but it scatters the truth
// across 8+ files. Whoever onboards a new service has to remember
// which indexes that service implicitly depends on. Putting them
// here means: one place to read, one place to change.
//
// What an index actually costs
// ----------------------------
//   - Write amplification — every insert/update/delete on an indexed
//     field has to update the index too. 4 indexes = 4 extra writes
//     per op. Matters for write-heavy collections, negligible for
//     auth/user data.
//   - RAM — Mongo wants the working set (hot indexes + active data)
//     to fit in memory. Index that overflows RAM = disk fallback =
//     latency cliff. Sum index size; budget against host memory.
//   - Hard cap — Mongo refuses to create more than 64 indexes on a
//     collection. In practice anything past 15 is a smell.
//
// Rule-of-thumb sizing
// --------------------
//   - 1–5 indexes per collection — perfectly normal.
//   - 5–10 — fine for collections with many query shapes.
//   - 15+ — review which indexes are actually used:
//       db.<col>.aggregate([{ $indexStats: {} }])
//
// When to add an index here
// -------------------------
// A query pattern that runs more than a handful of times per minute
// AND can't be served by an existing index. Adding "just in case"
// is the path to the cliff. Drop indexes nobody queries — Mongo
// reports usage in $indexStats.
//
// What NOT to index
// -----------------
//   - Boolean-only fields (only 2 buckets — almost no selectivity).
//   - Fields you only filter on once at startup or in admin tooling.
//   - Large/binary fields (extracted_text, blobs).
//
// ── Single-field indexes ──────────────────────────────────────────

// singleFieldIndexes lists every (collection, field, unique) tuple
// the platform expects. Grouped by domain so the rationale stays
// near the spec.
var singleFieldIndexes = []system.StorageIndexSpec{
	// Projects: scout dedupes by platform_id when ingesting; api
	// looks up by id on every project fetch.
	system.NewIndexSpec(constants.MongoProjectsCollection, "id", true),
	system.NewIndexSpec(constants.MongoProjectsCollection, "platform_id", true),

	// CVs: looked up by user_id on every /me/cv request and during
	// normalization. Without this, every lookup is a COLLSCAN
	// proportional to the catalogue size.
	system.NewIndexSpec(constants.MongoCVsCollection, "user_id", false),

	// Users: prefix-regex on email powers the /tokens autocomplete
	// in falcon-admin. Non-unique because legacy data may carry
	// duplicates — making it unique would make the Job fail before
	// it can create the rest of the indexes.
	system.NewIndexSpec(constants.MongoUsersCollection, "email", false),

	// Tokens: per-user views match by user_id (new) or email
	// (legacy, before user_id was stamped). Both halves of the OR
	// in /users/:id/tokens want the index.
	system.NewIndexSpec(constants.MongoTokensCollection, "user_id", false),
	system.NewIndexSpec(constants.MongoTokensCollection, "email", false),

	// APNs device tokens: signal sends a push by user_id; the admin
	// UI lists devices by user_id. device_id is unique because
	// signal upserts on it (one row per phone).
	system.NewIndexSpec(constants.MongoIOSDeviceTokensCollection, "user_id", false),
	system.NewIndexSpec(constants.MongoIOSDeviceTokensCollection, "device_id", true),

	// Realtime stats: dashboard rolls up by device_id, user_id,
	// event and recent created_at. All four are filters in the
	// realtime module's queries.
	system.NewIndexSpec(constants.MongoRealtimeStatsCollection, "device_id", false),
	system.NewIndexSpec(constants.MongoRealtimeStatsCollection, "user_id", false),
	system.NewIndexSpec(constants.MongoRealtimeStatsCollection, "event", false),
	system.NewIndexSpec(constants.MongoRealtimeStatsCollection, "created_at", false),

	// Normalized projects: looked up by project_id (1:1 with the
	// raw project) and by company_id (per-company drill-down). Sort
	// by display_updated_at on the matches feed.
	system.NewIndexSpec(constants.MongoNormalizedProjectsCollection, "project_id", true),
	system.NewIndexSpec(constants.MongoNormalizedProjectsCollection, "company_id", false),
	system.NewIndexSpec(constants.MongoNormalizedProjectsCollection, "display_updated_at", false),

	// Errors: triaged by service and (for scrape failures) by
	// platform. Used by /admin and the warnings dashboards.
	system.NewIndexSpec(constants.MongoErrorsCollection, "service_name", false),
	system.NewIndexSpec(constants.MongoErrorsCollection, "platform_id", false),
}

// ── TTL indexes (auto-delete on expiry) ───────────────────────────

type ttlIndexSpec struct{ Collection, Field string }

// ttlIndexes are single-field indexes with `expireAfterSeconds: 0`,
// which makes Mongo delete a document when the indexed datetime
// passes. Cheaper than a cron — the TTL monitor runs every 60s.
var ttlIndexes = []ttlIndexSpec{
	// Magic-link tokens have a hard 30-day expiry. Without this,
	// expired tokens accumulate forever.
	{constants.MongoTokensCollection, "expires_at"},
}

// ── Compound indexes ──────────────────────────────────────────────
//
// Reserved for query patterns that filter on field A AND sort/filter
// on field B in the same call. Order matters — Mongo can use a
// prefix of the index, so put the equality field first.
var compoundIndexes = []system.CompoundIndexSpec{
	// Match results: dedup by (cv_id, project_id) — the pair is the
	// natural key for one match. The (user_id, scored_at) compound
	// powers the user's match feed sorted by recency.
	{Collection: constants.MongoMatchResultsCollection, Fields: []string{"cv_id", "project_id"}, Unique: true},
	{Collection: constants.MongoMatchResultsCollection, Fields: []string{"user_id", "scored_at"}, Unique: false},

	// Companies: company_id is the canonical id used by storage and
	// api when caching the logo. Lives as a single-field "compound"
	// because the original call site used CompoundIndexSpec for
	// uniqueness — semantically equivalent to a unique single index.
	{Collection: constants.MongoCompaniesCollection, Fields: []string{"company_id"}, Unique: true},

	// User configurations: per-user, per-platform, per-device, per-
	// config-name uniqueness. The api upserts on this exact tuple so
	// the unique constraint is the invariant, not just an optimization.
	{Collection: constants.MongoUsersConfigurationsCollection, Fields: []string{"user_id", "platform", "device_id", "name"}, Unique: true},
}

// ensureAllIndexes runs every index spec through the storage helper.
// Returns the number of indexes acted on (created or already-present)
// and the first error encountered, if any. A failure on one index
// short-circuits the rest — better to surface the problem than to
// half-reconcile and exit 0.
func ensureAllIndexes(ctx context.Context) (int, error) {
	count := 0

	for _, spec := range singleFieldIndexes {
		if err := system.GetStorage().EnsureIndex(ctx, spec); err != nil {
			return count, fmt.Errorf("single %s.%s: %w", spec.Collection, spec.Field, err)
		}
		logrus.Infof("[config] index ok: %s.%s (unique=%v)", spec.Collection, spec.Field, spec.Unique)
		count++
	}

	for _, spec := range ttlIndexes {
		if err := system.GetStorage().EnsureTTLIndex(ctx, spec.Collection, spec.Field); err != nil {
			return count, fmt.Errorf("ttl %s.%s: %w", spec.Collection, spec.Field, err)
		}
		logrus.Infof("[config] ttl ok: %s.%s", spec.Collection, spec.Field)
		count++
	}

	for _, spec := range compoundIndexes {
		if err := system.GetStorage().EnsureCompoundIndex(ctx, spec); err != nil {
			return count, fmt.Errorf("compound %s on %v: %w", spec.Collection, spec.Fields, err)
		}
		logrus.Infof("[config] compound ok: %s on %v (unique=%v)", spec.Collection, spec.Fields, spec.Unique)
		count++
	}

	return count, nil
}
