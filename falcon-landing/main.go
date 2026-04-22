package main

import (
	"log"
	"net/http"
	"os"

	"hblabs.co/falcon/falcon-landing/landing"
)

func main() {
	if err := landing.Init(); err != nil {
		log.Fatalf("landing init: %v", err)
	}

	addr := ":" + portFromEnv()
	log.Printf("falcon-landing listening on %s", addr)

	if err := http.ListenAndServe(addr, landing.Handler()); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

// Default port matches the other Falcon HTTP services (k8s probes and
// the ingress target both assume 8080). Overridable via PORT for local
// dev collisions.
func portFromEnv() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8081"
}
