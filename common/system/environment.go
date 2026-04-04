package system

import (
	"log"
	"time"

	"github.com/joho/godotenv"
	"hblabs.co/falcon/common/helpers"
)

func LoadEnvs() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

// PollInterval reads POLL_INTERVAL from the environment (e.g. "30s", "5m").
// Defaults to 30s if unset or invalid.
func PollInterval() time.Duration {
	v := helpers.ReadEnvOptional("POLL_INTERVAL", "30s")
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
		Size:       helpers.ParseInt("BATCH_SIZE", 10),
		ItemDelay:  helpers.ParseDuration("BATCH_ITEM_DELAY", "2s"),
		BatchDelay: helpers.ParseDuration("BATCH_BATCH_DELAY", "10s"),
	}

	return cfg
}
