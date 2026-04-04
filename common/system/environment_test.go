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

func TestLoadEnvs_LoadsVariables(t *testing.T) {
	// Write a temporary .env file in the working directory (package dir during tests).
	const envFile = ".env"
	content := "TEST_LOAD_ENVS_VAR=expected_value\n"

	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to create .env file: %v", err)
	}
	t.Cleanup(func() {
		os.Remove(envFile)
		os.Unsetenv("TEST_LOAD_ENVS_VAR")
	})

	LoadEnvs()

	if got := os.Getenv("TEST_LOAD_ENVS_VAR"); got != "expected_value" {
		t.Errorf("TEST_LOAD_ENVS_VAR = %q, want %q", got, "expected_value")
	}
}
