// falcon-admin — backoffice gateway for falcon-nest.
//
// Single bearer-gated HTTP service that the dev portal proxies to
// for everything an operator needs to inspect or change about
// users, sessions, devices, magic links, configs, match results,
// realtime activity, and CV downloads. All MongoDB / MinIO access
// the admin UI needs flows through this binary; nest itself stays
// a thin shell.
//
// Not deployed to k8s today — runs locally with port-forwards to
// the cluster's Mongo (and, for CV downloads, to falcon-storage's
// NATS for the `cv.download.requested` request/reply).
//
//	kubectl port-forward -n falcon svc/mongo 27017:27017 &
//	MONGODB_URL=mongodb://localhost:27017 \
//	MONGODB_DATABASE=falcon \
//	ADMIN_TOKEN=change-me \
//	go run ./falcon-admin
//
// HTTP surface (every endpoint except /healthz requires
// `Authorization: Bearer <ADMIN_TOKEN>`):
//
//	GET    /healthz              readiness
//
//	POST   /test-link            issue a 30-day multi-use magic link
//	GET    /test-links           list every test token
//	DELETE /test-link/:id        revoke one test token
//	DELETE /test-links           purge every test token
//
//	GET    /stats                aggregate counters (registered users, …)
//	GET    /users/search         autocomplete by name or email
//	GET    /users/:id            per-user header info + counts
//	GET    /users/:id/tokens     magic links for this user
//	POST   /users/:id/tokens     mint a magic link for this user
//	DELETE /users/:id/tokens     revoke all magic links for this user
//	DELETE /tokens/:id           revoke one magic link
//	GET    /users/:id/sessions   active JWT sessions
//	DELETE /users/:id/sessions   sign the user out everywhere
//	DELETE /sessions/:id         revoke one session
//	GET    /users/:id/devices    APNs / push registrations
//	GET    /users/:id/cv         302-redirect to a presigned CV URL
//	GET    /users/:id/configs    UserConfig rows for this user
//	GET    /users/:id/matches    paginated match results (min_score filter)
//	GET    /users/:id/realtime   paginated realtime_stats events
package main

import (
	"fmt"
	"log"
	"net/http"

	"hblabs.co/falcon/admin/admin"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/system"
)

func main() {
	system.LoadEnvs()
	// Wire up the common system bits (Mongo, logger) before any
	// handler can dereference `system.GetStorage()`.
	system.Init()
	// NATS core only — admin doesn't subscribe to any streams, but
	// the /users/:id/cv endpoint needs to send a
	// `cv.download.requested` request to falcon-storage.
	system.InitBus(nil)

	port := environment.ParseInt("PORT", 8082)
	system.PrintStartupBannerAndPort(constants.ServiceAdmin, port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), admin.Handler()); err != nil {
		log.Fatalf("http: %v", err)
	}
}
