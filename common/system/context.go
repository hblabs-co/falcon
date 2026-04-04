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

// Init sets up the application context, cancelled on SIGINT or SIGTERM.
// Must be called once from main before spawning goroutines.
func Init() {
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
