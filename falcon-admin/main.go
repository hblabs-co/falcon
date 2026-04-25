// falcon-admin — backoffice gateway for falcon-nest.
//
// Single bearer-gated HTTP service that the dev portal proxies to
// for everything an operator needs to inspect or change about
// users, sessions, devices, magic links, configs, match results,
// realtime activity, and CV downloads.
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

	"hblabs.co/falcon/admin/admin"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/ownhttp"
	"hblabs.co/falcon/packages/system"
)

func main() {
	port := environment.ParseInt("PORT", 8082)
	ctx := system.Boot(constants.ServiceAdmin, system.WithPort(port))
	// NATS core only — admin doesn't subscribe to any streams, but
	// the /users/:id/cv endpoint needs to send a
	// `cv.download.requested` request to falcon-storage.
	system.InitBus(nil)

	srv := ownhttp.NewServerModule(constants.ServiceAdmin, fmt.Sprintf(":%d", port), admin.Handler())
	if err := system.RunForever(ctx, srv); err != nil {
		log.Fatalf("run: %v", err)
	}
}
