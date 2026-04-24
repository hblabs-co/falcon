package system

import (
	"errors"
	"io/fs"
	"log"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	environment "hblabs.co/falcon/common/environment"
)

// MustEnv returns the value of the required environment variable key.
// Calls logrus.Fatalf and exits if the variable is unset or empty.
func MustEnv(key string) string {
	v, err := environment.Read(key)
	if err != nil {
		logrus.Fatalf("%v", err)
	}
	return v
}

// LoadEnvs reads a local .env file into the process environment.
// Missing file is intentionally NOT fatal: in container / k8s
// deployments env vars come from the platform, not a file on disk.
// Any OTHER error (permission denied, malformed file) still fatals
// because that's a real misconfiguration.
func LoadEnvs() {
	err := godotenv.Load()
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return
	}
	log.Fatalf("error loading .env file: %v", err)
}

// Platform returns the PLATFORM environment variable. Fatals if not set.
// Used by multi-platform services (e.g. falcon-scout) to identify which platform
// this instance is responsible for.
func Platform() string {
	return MustEnv("PLATFORM")
}

// PollInterval reads POLL_INTERVAL from the environment (e.g. "30s", "5m").
// Defaults to 30s if unset or invalid.
func PollInterval() time.Duration {
	v := environment.ReadOptional("POLL_INTERVAL", "30s")
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("invalid POLL_INTERVAL %q, using 30s default", v)
		return 30 * time.Second
	}
	return d
}

// BatchConfig reads batch processing timing from environment variables:
//   - BATCH_SIZE:        number of items per batch before the long pause (default 5)
//   - BATCH_ITEM_DELAY:  pause between items within a batch, e.g. "1s" (default 3s)
//   - BATCH_BATCH_DELAY: longer pause between batches, e.g. "10s" (default 15s)
func BatchCfg() BatchConfig {
	cfg := BatchConfig{
		Size:       environment.ParseInt("BATCH_SIZE", 10),
		ItemDelay:  environment.ParseDuration("BATCH_ITEM_DELAY", "2s"),
		BatchDelay: environment.ParseDuration("BATCH_BATCH_DELAY", "10s"),
	}

	return cfg
}
