package system

import (
	"context"
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

// initWithStorage sets up the signal-cancellable context and connects
// to MongoDB. Internal helper for Boot — services don't call it
// directly (and main()s that skip Mongo go through initWithoutStorage
// via Boot's WithoutStorage option).
func initWithStorage() {
	initWithoutStorage()
	if err := InitStorage(Ctx()); err != nil {
		logrus.Fatalf("storage init failed: %v", err)
	}
}

// initWithoutStorage sets up the signal-cancellable context and logs
// BUILD_TIME, but skips Mongo. Used by Boot's WithoutStorage option
// for local dev tools and CLIs that don't need persistence.
func initWithoutStorage() {
	appCtx, appStop = signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
}
