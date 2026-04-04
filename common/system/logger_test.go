package system

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestPidHook_Levels(t *testing.T) {
	hook := &PidHook{Pid: 1}
	levels := hook.Levels()
	if len(levels) != len(logrus.AllLevels) {
		t.Errorf("Levels() returned %d levels, want %d", len(levels), len(logrus.AllLevels))
	}
}

func TestPidHook_Fire(t *testing.T) {
	const wantPid = 12345
	hook := &PidHook{Pid: wantPid}

	entry := logrus.NewEntry(logrus.StandardLogger())
	err := hook.Fire(entry)
	if err != nil {
		t.Fatalf("Fire returned unexpected error: %v", err)
	}

	pid, ok := entry.Data["pid"]
	if !ok {
		t.Fatal(`entry.Data["pid"] was not set`)
	}
	if pid != wantPid {
		t.Errorf("pid = %v, want %d", pid, wantPid)
	}
}

func TestPidHook_Fire_UsesPidField(t *testing.T) {
	hook := &PidHook{Pid: os.Getpid()}
	entry := logrus.NewEntry(logrus.StandardLogger())

	if err := hook.Fire(entry); err != nil {
		t.Fatalf("Fire returned error: %v", err)
	}
	if entry.Data["pid"] != os.Getpid() {
		t.Errorf("expected pid %d, got %v", os.Getpid(), entry.Data["pid"])
	}
}

func TestConfigtLogger_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ConfigtLogger panicked: %v", r)
		}
	}()
	ConfigLogger()
}
