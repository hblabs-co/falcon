// falcon-authorizer — local-only admin helper for issuing long-lived
// magic-link tokens used by App Store reviewers and manual QA.
//
// Not deployed to k8s. Run it locally after opening a port-forward to
// the cluster's Mongo, so the tokens it creates land in the same
// collection falcon-api reads from.
//
//	kubectl port-forward -n falcon svc/mongo 27017:27017 &
//	MONGODB_URL=mongodb://localhost:27017 \
//	MONGODB_DATABASE=falcon \
//	AUTHORIZER_TOKEN=change-me \
//	go run ./falcon-authorizer
//
// HTTP:
//
//	POST   /test-link        create a new 30-day multi-use link
//	GET    /test-links       list active test tokens
//	DELETE /test-link/:id    revoke one test token
//	DELETE /test-links       purge every test token at once
//	GET    /healthz          readiness
//
// All endpoints (except /healthz) require `Authorization: Bearer <AUTHORIZER_TOKEN>`.
package main

import (
	"fmt"
	"log"
	"net/http"

	"hblabs.co/falcon/common/constants"
	environment "hblabs.co/falcon/common/environment"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/falcon-authorizer/authorizer"
)

func main() {
	system.LoadEnvs()
	// Wire up the common system bits (Mongo, logger) before any
	// handler can dereference `system.GetStorage()`.
	system.Init()

	port := environment.ParseInt("PORT", 8082)
	system.PrintStartupBannerAndPort(constants.ServiceAuthorizer, port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), authorizer.Handler()); err != nil {
		log.Fatalf("http: %v", err)
	}
}
