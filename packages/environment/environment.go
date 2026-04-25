package environment

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

// loadOnce makes sure godotenv.Load() runs at most once, on the first
// public Read call. Eliminates the old "remember to call LoadEnvs()
// before any env access" foot-gun: every Read here gates on this and
// the .env file is loaded automatically.
//
// In container / k8s deployments .env doesn't exist (env vars come
// from the configmap/secret directly into the process); fs.ErrNotExist
// is treated as "no .env to merge" and silently ignored. Other errors
// (permission denied, malformed file) still fatal — those represent a
// real misconfiguration, not a clean container boot.
var loadOnce sync.Once

func ensureLoaded() {
	loadOnce.Do(func() {
		err := godotenv.Load()
		if err == nil || errors.Is(err, fs.ErrNotExist) {
			return
		}
		log.Fatalf("error loading .env file: %v", err)
	})
}

// Read returns the value of the environment variable key with whitespace trimmed.
// Returns an error if the variable is unset or empty.
func Read(key string) (string, error) {
	ensureLoaded()
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return "", fmt.Errorf("required environment variable not set: %s", key)
	}
	return val, nil
}

func ReadMany(keys ...string) ([]string, error) {
	values := make([]string, len(keys))
	var missing []string

	for i, key := range keys {
		val, err := Read(key)
		if err != nil {
			missing = append(missing, key)
			continue
		}
		values[i] = val
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing env vars: %v", missing)
	}

	return values, nil
}

// ReadOptional returns the value of the environment variable key with whitespace
// trimmed, or defaultVal if it is unset or empty.
func ReadOptional(key string, defaultVal string) string {
	ensureLoaded()
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	return val
}

func ParseInt(key string, def int) int {
	v := ReadOptional(key, "")
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		logrus.Printf("invalid %s %q, using %d default", key, v, def)
		return def
	}
	return n
}

func ParseFloat32(key string, def float32) float32 {
	v := ReadOptional(key, "")
	if v == "" {
		return def
	}
	var f float64
	if _, err := fmt.Sscanf(v, "%f", &f); err != nil {
		logrus.Printf("invalid %s %q, using %v default", key, v, def)
		return def
	}
	return float32(f)
}

func ParseDuration(key, def string) time.Duration {
	v := ReadOptional(key, def)
	d, err := time.ParseDuration(v)
	if err != nil {
		logrus.Printf("invalid %s %q, using %s default", key, v, def)
		d, _ = time.ParseDuration(def)
	}
	return d
}
