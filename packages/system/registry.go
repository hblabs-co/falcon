package system

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
)

// RegisterService upserts a metadata document for the given service in
// the `system` collection so clients (and ops) can ask "when was this
// service last published?" via the public GET /system endpoint.
//
// Called once from each service's main after InitStorage. Services
// that don't touch Mongo (admin, import) skip this call and
// simply don't appear in /system responses.
//
// publishDate is normally injected at build time (ldflags / env var)
// so restarts of the same binary preserve the original publish date.
// A zero value means "the service has no declared publish date" —
// the helper stamps time.Now() as a fallback. On every startup the
// updated_at field is rewritten regardless, so clients can also read
// it as a "service was last alive at" heartbeat-ish signal.
//
// Idempotent: same service_name, repeated calls, no duplicates.
func RegisterService(ctx context.Context, serviceName string, publishDate time.Time) error {
	if serviceName == "" {
		return fmt.Errorf("register service: empty service name")
	}
	if publishDate.IsZero() {
		publishDate = time.Now()
	}
	now := time.Now()

	// Upsert keyed on service_name. $setOnInsert keeps the original
	// publish_date on restart — only the first write (or an explicit
	// new build-time value) reshapes it.
	filter := bson.M{constants.SystemFieldServiceName: serviceName}
	update := bson.M{
		"$setOnInsert": bson.M{
			constants.SystemFieldPublishDate: publishDate,
		},
		"$set": bson.M{
			constants.SystemFieldServiceName: serviceName,
			constants.SystemFieldUpdatedAt:   now,
		},
	}

	if err := GetStorage().RawUpdate(ctx, constants.MongoSystemCollection, filter, update); err != nil {
		return fmt.Errorf("register service %s: %w", serviceName, err)
	}
	return nil
}

// RegisterServiceFromBuildTime is a convenience wrapper that reads the
// BUILD_TIME env var (RFC3339, the same format container images are
// tagged with) and passes it as publishDate to RegisterService. Falls
// back to time.Now() when BUILD_TIME is missing or unparseable, so
// local dev (where the var is never set) still gets a meaningful
// value without every main having to branch on os.Getenv.
//
// Logs a warning but doesn't return the registration error — a
// transient Mongo hiccup at boot shouldn't take the whole service
// down over non-critical metadata.
func RegisterServiceFromBuildTime(ctx context.Context, serviceName string) {
	var publishDate time.Time
	if raw := os.Getenv("BUILD_TIME"); raw != "" {
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			publishDate = t
		} else {
			logrus.Warnf("register service %s: BUILD_TIME %q is not RFC3339, falling back to now", serviceName, raw)
		}
	}
	if err := RegisterService(ctx, serviceName, publishDate); err != nil {
		logrus.Warnf("register service %s: %v", serviceName, err)
	}
}
