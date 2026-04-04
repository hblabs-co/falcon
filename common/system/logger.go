package system

import (
	"os"

	"github.com/sirupsen/logrus"
)

type PidHook struct {
	Pid int
}

func (h *PidHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *PidHook) Fire(entry *logrus.Entry) error {
	entry.Data["pid"] = h.Pid
	return nil
}

func ConfigLogger() {
	logrus.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	logrus.AddHook(&PidHook{Pid: os.Getpid()})
}
