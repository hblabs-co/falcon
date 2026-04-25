package system

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
)

// Boot consolidates the boilerplate prelude every Falcon service was
// repeating in its main(): ConfigLogger, Init (ctx + Mongo), banner,
// and self-registration in the `system` collection. .env loading is
// handled automatically by packages/environment on first access, so
// it doesn't appear here.
//
// One call instead of five, in a fixed order, returns the
// signal-cancellable context. Use opts for the small variations:
//
//	ctx := system.Boot(constants.ServiceAdmin, system.WithPort(8082))
//	ctx := system.Boot(constants.ServiceScout)
//	ctx := system.Boot(constants.ServiceDesigner,
//	    system.WithoutStorage(),                  // no Mongo
//	    system.WithoutRegistration(),             // not in /system listing
//	    system.WithPort(8083),
//	    system.WithBannerExtras("watching: …"))
//
// What it does NOT do: InitBus. Streams vary per service and should
// stay in main() so the operator sees them at a glance. Call InitBus
// after Boot, before RunForever.
func Boot(service string, opts ...BootOpt) context.Context {
	cfg := bootConfig{
		registerInSystem: true,
		initStorage:      true,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	ConfigLogger()
	// Set up BUILD_TIME log + signal-cancellable context. Most
	// services also want Mongo; pure CLIs and the designer dev tool
	// pass WithoutStorage to skip that part. .env auto-loads on first
	// env access via packages/environment — no explicit step needed.
	if cfg.initStorage {
		initWithStorage()
	} else {
		initWithoutStorage()
	}

	if cfg.port > 0 {
		PrintStartupBannerAndPort(service, cfg.port, cfg.bannerExtras...)
	} else {
		PrintStartupBanner(service, cfg.bannerExtras...)
	}

	if cfg.registerInSystem && cfg.initStorage {
		RegisterServiceFromBuildTime(Ctx(), service)
	}

	// BUILD_TIME is baked into the image by docker-bake (see
	// docker-bake.hcl). Logging it first thing on startup lets you tell
	// from the pod's logs whether it's running the image you just
	// pushed, or an older cached one — invaluable when a rollout
	// "didn't take" and you're not sure why.
	buildTime := os.Getenv("BUILD_TIME")
	if buildTime == "" {
		buildTime = "unknown"
	}
	logrus.Infof("image built at %s", buildTime)

	return Ctx()
}

// BootOpt configures Boot. Defaults: storage on, registration on,
// no port (banner skips URL lines), no extras.
type BootOpt func(*bootConfig)

type bootConfig struct {
	port             int
	bannerExtras     []string
	registerInSystem bool
	initStorage      bool
}

// WithPort tells the banner to print loopback + LAN URLs at this port.
// The HTTP listener itself is still the caller's job (or
// ownhttp.NewServerModule).
func WithPort(port int) BootOpt {
	return func(c *bootConfig) { c.port = port }
}

// WithBannerExtras appends arbitrary lines to the boxed banner —
// "watching: …", "config: …", any human-readable startup hint.
func WithBannerExtras(lines ...string) BootOpt {
	return func(c *bootConfig) { c.bannerExtras = append(c.bannerExtras, lines...) }
}

// WithoutStorage skips Mongo initialisation. Use for local dev tools
// (designer) or one-shot CLIs that don't touch the DB. Implies
// WithoutRegistration since /system rows live in Mongo.
func WithoutStorage() BootOpt {
	return func(c *bootConfig) {
		c.initStorage = false
		c.registerInSystem = false
	}
}

// WithoutRegistration skips the `system` collection upsert. Useful
// when a service shouldn't appear in the /system listing (e.g.
// transient migration jobs).
func WithoutRegistration() BootOpt {
	return func(c *bootConfig) { c.registerInSystem = false }
}
