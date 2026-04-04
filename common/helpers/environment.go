package helpers

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

// ReadEnv returns the value of the environment variable key with whitespace trimmed.
// Panics if the variable is unset or empty.
func ReadEnv(key string) (string, error) {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return "", fmt.Errorf("required environment variable not set: %s", key)
	}
	return val, nil
}

func ReadEnvs(keys ...string) ([]string, error) {
	values := make([]string, len(keys))
	var missing []string

	for i, key := range keys {
		val, err := ReadEnv(key)
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

// ReadEnvOptional returns the value of the environment variable key with whitespace
// trimmed, or defaultVal if it is unset or empty.
func ReadEnvOptional(key string, defaultVal string) string {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return defaultVal
	}
	return val
}

func ParseInt(key string, def int) int {
	v := ReadEnvOptional(key, "")
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
	v := ReadEnvOptional(key, "")
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
	v := ReadEnvOptional(key, def)
	d, err := time.ParseDuration(v)
	if err != nil {
		logrus.Printf("invalid %s %q, using %s default", key, v, def)
		d, _ = time.ParseDuration(def)
	}
	return d
}
