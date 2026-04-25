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
	system.LoadEnvs()
	system.ConfigLogger()
	system.Init()

	system.PrintStartupBanner(constants.ServiceConfig)

	created, err := ensureAllIndexes(system.Ctx())
	if err != nil {
		logrus.Errorf("config bootstrap failed: %v", err)
		os.Exit(1)
	}
	logrus.Infof("[config] bootstrap done — %d index(es) reconciled", created)
}
