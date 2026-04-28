package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// ensureUserLastLogin backfills users.last_logged_in_at for any user
// who was created before the field existed but currently holds at
// least one live JWT row in the tokens collection.
//
// Why we need this: tokens.expires_at has a TTL index — Mongo
// auto-deletes JWTs once they pass their expiry. So once a user's
// JWT TTLs out, we lose ALL evidence in the DB that they ever
// signed in (no JWT row, no jwt-issued audit trail, nothing). The
// new login-reminder loop would then mistake them for "never
// onboarded" users and start emailing them.
//
// This migration runs once per boot of falcon-config and is safe to
// repeat: it only touches users that lack the field. Live updates
// from this point are handled by /auth/verify in falcon-api.
//
// Two-pass to keep memory bounded:
//
//  1. Distinct user_ids from tokens where type=jwt → set A.
//     Cheap because tokens.user_id is indexed and the type filter is
//     selective.
//  2. For users with last_logged_in_at missing AND id in A: set
//     last_logged_in_at to NOW (we don't have the original verify
//     timestamp; the field is "evidence they did sign in", not
//     "exactly when"). NOW is fine because the cadence loop only
//     checks IsZero.
//
// Returns the count of backfilled rows for the bootstrap log.
func ensureUserLastLogin(ctx context.Context) (int64, error) {
	storage := system.GetStorage()

	userIDs, err := storage.Distinct(ctx,
		constants.MongoTokensCollection,
		"user_id",
		bson.M{"type": "jwt", "user_id": bson.M{"$ne": ""}},
	)
	if err != nil {
		return 0, fmt.Errorf("distinct jwt user_ids: %w", err)
	}
	if len(userIDs) == 0 {
		logrus.Info("[config] ensureUserLastLogin — no live JWTs found, nothing to backfill")
		return 0, nil
	}

	const batchSize = 1000
	now := bson.M{"$currentDate": bson.M{"last_logged_in_at": true, "updated_at": true}}
	var backfilled int64
	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		ids := userIDs[i:end]
		// Only touch users where the field is genuinely missing —
		// don't overwrite a more accurate value already set by an
		// /auth/verify since boot.
		res, err := storage.BulkUpdate(ctx,
			constants.MongoUsersCollection,
			bson.M{
				"id":                bson.M{"$in": ids},
				"last_logged_in_at": bson.M{"$exists": false},
			},
			now,
		)
		if err != nil {
			return backfilled, fmt.Errorf("backfill last_logged_in_at batch %d-%d: %w", i, end, err)
		}
		backfilled += res
	}

	logrus.Infof("[config] ensureUserLastLogin — backfilled %d user(s) from %d distinct JWT user_ids",
		backfilled, len(userIDs))
	return backfilled, nil
}
