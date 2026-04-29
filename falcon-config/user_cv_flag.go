package main

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// ensureUserCVFlag reconciles the `users.cv_uploaded` denormalisation
// against the source of truth in the `cvs` collection. Runs once per
// boot of falcon-config; idempotent.
//
// Two-pass migration:
//
//  1. Backfill: every `users` doc that lacks the cv_uploaded field
//     gets it set to false. New users created from now on already
//     have the field (Go's zero-value is false), so this only ever
//     touches pre-existing rows from before the field was added.
//
//  2. Reconcile: walk the indexed CVs, mark every distinct user_id
//     as cv_uploaded=true. Anyone who actually has a CV but ended
//     up flagged false in step 1 gets fixed here. Live updates after
//     this point are handled by falcon-storage flipping the flag at
//     index time — see falcon-storage/cv/service.go.
//
// Returns the totals so the bootstrap log shows what changed.
func ensureUserCVFlag(ctx context.Context) (backfilled, reconciled int64, err error) {
	storage := system.GetStorage()

	// Pass 1 — set cv_uploaded:false on users missing the field.
	res, err := storage.BulkUpdate(ctx,
		constants.MongoUsersCollection,
		bson.M{"cv_uploaded": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"cv_uploaded": false}},
	)
	if err != nil {
		return 0, 0, fmt.Errorf("backfill cv_uploaded=false: %w", err)
	}
	backfilled = res

	// Pass 2 — pull every distinct user_id from `cvs` where the doc
	// is fully indexed, then bulk-update those users to true.
	// Distinct keeps the payload small (one entry per user even if
	// they have multiple CVs); BulkUpdate flips the flag in one round
	// trip. Done in batches of 1k to stay friendly with Mongo's
	// document size limit on the $in filter.
	// "Has a usable CV" = anything past the indexing pipeline. The
	// canonical set lives in models.CVStatusesUsable so this query
	// stays in lockstep with the cv-reminder recheck and any other
	// caller asking "does this user have a CV ready?". Earlier
	// versions hardcoded `["indexed"]` here and silently missed
	// every CV the normalizer had already advanced — the historic
	// false-positive reminders trace back to that single point of
	// drift, which is now impossible.
	cvReadyStatuses := bson.M{"$in": models.CVStatusesUsableBSON()}

	userIDs, err := storage.Distinct(ctx,
		constants.MongoCVsCollection,
		"user_id",
		bson.M{
			"status":  cvReadyStatuses,
			"user_id": bson.M{"$ne": ""},
		},
	)
	if err != nil {
		return backfilled, 0, fmt.Errorf("distinct cv user_ids: %w", err)
	}

	const batchSize = 1000
	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		ids := userIDs[i:end]
		res, err := storage.BulkUpdate(ctx,
			constants.MongoUsersCollection,
			bson.M{
				"id":          bson.M{"$in": ids},
				"cv_uploaded": bson.M{"$ne": true},
			},
			bson.M{"$set": bson.M{"cv_uploaded": true}},
		)
		if err != nil {
			return backfilled, reconciled, fmt.Errorf("flip cv_uploaded=true batch %d-%d: %w", i, end, err)
		}
		reconciled += res
	}

	logrus.Infof("[config] ensureUserCVFlag — backfilled %d (set false), reconciled %d (set true)",
		backfilled, reconciled)
	return backfilled, reconciled, nil
}
