package main

import (
	"fmt"
	"log"
	"net/http"

	"hblabs.co/falcon/landing/landing"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/system"
)

func main() {

	system.LoadEnvs()
	if err := landing.Init(); err != nil {
		log.Fatalf("landing init: %v", err)
	}

	// Default port matches the other Falcon HTTP services; k8s probes
	// and the ingress target both assume 8081. Overridable via PORT
	// for local dev collisions.
	port := environment.ParseInt("PORT", 8081)
	system.PrintStartupBannerAndPort(constants.ServiceLanding, port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), landing.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}
