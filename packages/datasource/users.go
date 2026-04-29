// Package datasource is the domain-level data access layer for
// Falcon. One file per MongoDB collection (users.go, cvs.go, ...)
// — each exposes the queries that several services share, so the
// rules around a collection live in one place instead of scattered
// across falcon-api, falcon-admin, falcon-signal.
//
// Distinct from `packages/system.Storage`, which is the low-level
// Mongo client (Get / Set / FindPage / Count). This package is the
// domain-aware repo on top: "find user by email", "find-or-create
// with the right race semantics", "list active CVs for user".
//
// Email parameters are NOT normalised here. Callers are expected
// to pass the canonical (lowercased + trimmed) email; doing it
// here would silently disagree with whatever the caller used to
// store the row in the first place.
package datasource

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// NormalizeEmail returns the canonical form used everywhere in the
// stack: lowercased + trimmed of surrounding whitespace. Apply at
// the boundary (request bind, query param, NATS event) so every
// downstream lookup, throttle bucket, opt-out row, and admin block
// agree on what "the same email" means. Doing it here means
// `Helmer@HBLabs.CO ` and `helmer@hblabs.co` collapse to the same
// canonical user.
//
// NOT a full RFC-5321 normaliser — we don't strip Gmail-style
// `+tags` or domain-case anywhere besides the host. RFC 5321 says
// the local part is technically case-sensitive; in practice no
// real provider treats it as such, and treating two casings as
// the same user is the lesser evil.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// FindUserByEmail returns the user with the given email. Returns
// `mongo.ErrNoDocuments` if no row matches — callers can
// `errors.Is` against it to differentiate "not found" from a real
// Mongo error. Email is normalised here so a caller that forgot
// still hits the canonical row.
func FindUserByEmail(ctx context.Context, email string) (models.User, error) {
	var u models.User
	err := system.GetStorage().GetByField(ctx, constants.MongoUsersCollection, "email", NormalizeEmail(email), &u)
	return u, err
}

// FindUserByID returns the user with the given gonanoid id. Returns
// `mongo.ErrNoDocuments` if no row matches — same contract as
// FindUserByEmail. Centralised here so the admin handlers that load
// users by `:id` URL param share a single primitive instead of
// each domain package re-doing the GetById call.
func FindUserByID(ctx context.Context, id string) (models.User, error) {
	var u models.User
	err := system.GetStorage().GetById(ctx, constants.MongoUsersCollection, id, &u)
	return u, err
}

// UserExistsByEmail returns true iff a user with the given email
// exists. Swallows ErrNoDocuments and any other Mongo error —
// useful for snapshot/context-style checks where a transient
// failure shouldn't poison the result. If you need to distinguish
// "not found" from "Mongo down", use FindUserByEmail directly.
func UserExistsByEmail(ctx context.Context, email string) bool {
	_, err := FindUserByEmail(ctx, email)
	return err == nil
}

// FindOrCreateUser returns the existing user_id for the given email or
// creates a new user (with a fresh nanoid) and returns its id.
//
// Race-safe: with the unique index on `users.email`, a concurrent
// FindOrCreateUser either wins the Insert (returns its new id) or
// loses with a duplicate-key error and re-fetches the winner's id.
// Either way both callers end up returning the SAME canonical id —
// no orphan User docs, no JWTs pointing at non-existent users
// (reports.md B3).
//
// `Insert` (not upsert) is the right primitive here: an existing
// user's other fields must NOT be overwritten on a "find" call.
func FindOrCreateUser(ctx context.Context, email string) (string, error) {
	email = NormalizeEmail(email)
	if u, err := FindUserByEmail(ctx, email); err == nil {
		return u.ID, nil
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		return "", err
	}

	now := time.Now()
	u := models.User{
		ID:        gonanoid.Must(),
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := system.GetStorage().Insert(ctx, constants.MongoUsersCollection, u); err != nil {
		// Race lost: another caller inserted the user between our
		// FindUserByEmail and Insert. Re-fetch and return its id.
		if mongo.IsDuplicateKeyError(err) {
			existing, ferr := FindUserByEmail(ctx, email)
			if ferr != nil {
				return "", fmt.Errorf("dup-key on insert but re-find failed: %w", ferr)
			}
			return existing.ID, nil
		}
		return "", err
	}
	return u.ID, nil
}
