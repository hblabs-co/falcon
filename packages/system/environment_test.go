package system

import (
	"os"
	"testing"
	"time"
)

func TestPollInterval_Default(t *testing.T) {
	os.Unsetenv("POLL_INTERVAL")
	if got := PollInterval(); got != 30*time.Second {
		t.Errorf("PollInterval() = %v, want 30s", got)
	}
}

func TestPollInterval_FromEnv(t *testing.T) {
	os.Setenv("POLL_INTERVAL", "5m")
	t.Cleanup(func() { os.Unsetenv("POLL_INTERVAL") })
	if got := PollInterval(); got != 5*time.Minute {
		t.Errorf("PollInterval() = %v, want 5m", got)
	}
}

func TestPollInterval_InvalidFallsBackToDefault(t *testing.T) {
	os.Setenv("POLL_INTERVAL", "not-a-duration")
	t.Cleanup(func() { os.Unsetenv("POLL_INTERVAL") })
	if got := PollInterval(); got != 30*time.Second {
		t.Errorf("PollInterval() = %v, want 30s on invalid input", got)
	}
}

