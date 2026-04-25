package main

import (
	"fmt"
	"log"

	"hblabs.co/falcon/landing/landing"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/ownhttp"
	"hblabs.co/falcon/packages/system"
)

func main() {
	port := environment.ParseInt("PORT", 8081)
	ctx := system.Boot(constants.ServiceLanding,
		system.WithPort(port),
		system.WithoutStorage(), // landing is stateless static-content
	)
	if err := landing.Init(); err != nil {
		log.Fatalf("landing init: %v", err)
	}

	srv := ownhttp.NewServerModule(constants.ServiceLanding, fmt.Sprintf(":%d", port), landing.Handler())
	if err := system.RunForever(ctx, srv); err != nil {
		log.Fatalf("run: %v", err)
	}
}
