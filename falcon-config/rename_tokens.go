package main

// One-shot migration: rename the legacy `tokens` collection to
// `auth_tokens` so it lines up with `auth_intents`, `auth_blocks`,
// and `auth_optouts`. The auth domain owns four collections; only
// `tokens` predated the prefix convention.
//
// Idempotent on every boot:
//   - Fresh cluster (neither name present) → no-op; the first write
//     creates `auth_tokens` directly.
//   - Already migrated (`auth_tokens` present) → no-op.
//   - Mid-migration (only `tokens` present) → rename via Mongo's
//     admin command, atomic, preserves indexes.
//
// Runs BEFORE ensureAllIndexes so the index specs (which use the
// new `MongoAuthTokensCollection = "auth_tokens"` constant) apply to
// the renamed collection. Without that order, EnsureIndex would
// create indexes on a brand-new empty `auth_tokens` while the
// real data still lived under the old name.
//
// Coordination: the redeploy gate (`kubectl wait
// --for=condition=complete job/falcon-config`) blocks every
// service Deployment until this finishes — so no falcon-api or
// falcon-admin pod can write to `tokens` after the rename has
// completed but before the new code rolls out.

import (
	"context"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// legacyTokensCollection is the pre-rename name. Hard-coded here on
// purpose — the constants package now points at the new name, so
// this is the only place that still references the old one.
const legacyTokensCollection = "tokens"

func renameLegacyTokens(ctx context.Context) error {
	if err := system.GetStorage().RenameCollection(ctx,
		legacyTokensCollection,
		constants.MongoAuthTokensCollection,
	); err != nil {
		return err
	}
	logrus.Infof("[config] rename %s → %s ok (no-op if already migrated or fresh DB)",
		legacyTokensCollection, constants.MongoAuthTokensCollection)
	return nil
}
