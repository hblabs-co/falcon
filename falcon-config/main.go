// falcon-config — one-shot bootstrap job for shared infrastructure
// configuration that has to be in place before the long-running
// services come up. Today that's the MongoDB index list; tomorrow
// it could be NATS streams, Mongo collation defaults, seed data, or
// anything else that's idempotent and needs to land once per
// environment.
//
// Designed to run as a Kubernetes Job; can also be invoked locally
// via `go run ./falcon-config`. Every step is idempotent so running
// the job after services are already up is a safe no-op.
//
// Exit codes:
//
//	0   every step succeeded (or was already in the desired state)
//	1   at least one step returned an error
//
// Environment:
//
//	MONGODB_URL       e.g. mongodb://localhost:27017
//	MONGODB_DATABASE  e.g. falcon
//
// Each concern lives in its own sibling file (today: indexes.go).
package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

func main() {
	// One-shot Job — no NATS, no HTTP listener, no shutdown phase.
	// WithoutRegistration because the bootstrap job isn't a long-running
	// service; it shouldn't appear in the /system listing as if it were.
	ctx := system.Boot(constants.ServiceConfig, system.WithoutRegistration())

	created, err := ensureAllIndexes(ctx)
	if err != nil {
		logrus.Errorf("config bootstrap failed: %v", err)
		os.Exit(1)
	}
	logrus.Infof("[config] bootstrap done — %d index(es) reconciled", created)


	// Backfill users.last_logged_in_at for users with a currently-
	// live JWT but missing the field. Without this, the login-
	// reminder loop would re-target users whose first login pre-
	// dates the field, since their JWT row would TTL out and we'd
	// lose the only evidence they ever signed in.
	if _, err := ensureUserLastLogin(ctx); err != nil {
		logrus.Errorf("ensureUserLastLogin failed: %v", err)
		os.Exit(1)
	}
}
