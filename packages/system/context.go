package system

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/sirupsen/logrus"
)

var (
	appCtx  context.Context
	appStop context.CancelFunc
)

// Ctx returns the application context created by Init.
func Ctx() context.Context {
	return appCtx
}

// Init sets up the application context, cancelled on SIGINT or SIGTERM.
// Must be called once from main before spawning goroutines.
func Init() {
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

	appCtx, appStop = signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)

	if err := InitStorage(Ctx()); err != nil {
		logrus.Fatalf("storage init failed: %v", err)
	}
}

// Wait blocks until the application context is cancelled, then releases signal resources.
func Wait() {
	<-appCtx.Done()
	appStop()
}
