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

	// Users: email is the natural key for "is this person already
	// a user?" — used by `datasource.FindOrCreateUser` and the
	// auth_intents context snapshot. Unique guarantees no
	// concurrent verifies create two `User` rows for the same
	// email; FindOrCreateUser handles the duplicate-key error by
	// re-fetching the winner's id (reports.md B3). If the live DB
	// has historical duplicates, dedupe them BEFORE applying this
	// index — the Job fails fast otherwise.
	system.NewIndexSpec(constants.MongoUsersCollection, "email", true),
	// `id` is the gonanoid primary key stored in the `id` BSON field
	// (NOT `_id` — the default `_id` index doesn't help us). Hit on
	// every login by `handleVerify` (UpdateOne {id: userID} for
	// last_logged_in_at) and across falcon-api/me when looking up
	// "who is this JWT's subject?". Without this, every UpdateOne
	// is a COLLSCAN (reports.md N4).
	system.NewIndexSpec(constants.MongoUsersCollection, "id", true),

	// Tokens: per-user views match by user_id (new) or email
	// (legacy, before user_id was stamped). Both halves of the OR
	// in /users/:id/tokens want the index. token_hash is unique —
	// every magic-link is a distinct nanoid → SHA-256 hash, and
	// /auth/verify hits this index on every login (without it the
	// query is a collection scan). Unique also adds defense in
	// depth against the single-use race fixed by the CAS in
	// verify.go.
	system.NewIndexSpec(constants.MongoAuthTokensCollection, "user_id", false),
	system.NewIndexSpec(constants.MongoAuthTokensCollection, "email", false),
	system.NewIndexSpec(constants.MongoAuthTokensCollection, "token_hash", true),
	// `id` is the gonanoid primary key stored in the `id` BSON
	// field. Hit on every login by the CAS in `handleVerify`
	// (UpdateOne {id, used:false}) and by `issueJWT` (Set {id:
	// tokenID}). Without this, both filters are COLLSCANs that
	// grow with the 30-day JWT TTL window (reports.md N4).
	system.NewIndexSpec(constants.MongoAuthTokensCollection, "id", true),

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

	// Auth intents: append-only log of magic-link requests. Queries
	// hit by email (customer support, abuse), by client.ip (abuse),
	// by device_id (correlated abuse), and by recency. IP is now
	// nested under the embedded clientmeta.ClientMeta — see
	// packages/clientmeta. Caveat: IP isn't 1:1 with users (NAT,
	// CGNAT, VPN); the index helps aggregations and forensic
	// lookups but the abuse threshold must be tuned for shared IPs.
	system.NewIndexSpec(constants.MongoAuthIntentsCollection, "email", false),
	system.NewIndexSpec(constants.MongoAuthIntentsCollection, "client.ip", false),
	system.NewIndexSpec(constants.MongoAuthIntentsCollection, "device_id", false),
	system.NewIndexSpec(constants.MongoAuthIntentsCollection, "requested_at", false),

	// Auth blocks: lookup by email is the hot path (every magic-link
	// request checks this collection). Unique because we want at most
	// one active block row per email.
	system.NewIndexSpec(constants.MongoAuthBlocksCollection, "email", true),
}

// ── TTL indexes (auto-delete on expiry) ───────────────────────────

type ttlIndexSpec struct{ Collection, Field string }

// ttlIndexes are single-field indexes with `expireAfterSeconds: 0`,
// which makes Mongo delete a document when the indexed datetime
// passes. Cheaper than a cron — the TTL monitor runs every 60s.
var ttlIndexes = []ttlIndexSpec{
	// Magic-link tokens have a hard 30-day expiry. Without this,
	// expired tokens accumulate forever.
	{constants.MongoAuthTokensCollection, "expires_at"},
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

	// User reminders: one row per (user_id, kind). The cv-reminder
	// loop in falcon-signal upserts on this pair every time it sends,
	// and selects against `kind + stopped` to skip terminated users.
	{Collection: constants.MongoUserRemindersCollection, Fields: []string{"user_id", "kind"}, Unique: true},

	// Auth opt-outs: one row per (email, kind). The reminder loops
	// bulk-fetch with email $in + kind once per page of candidates;
	// the compound covers that exact filter shape and enforces "at
	// most one opt-out row per (email, kind)".
	{Collection: constants.MongoAuthOptOutsCollection, Fields: []string{"email", "kind"}, Unique: true},
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
