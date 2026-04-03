package helpers

import (
	"fmt"
	"os"
	"strings"
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
