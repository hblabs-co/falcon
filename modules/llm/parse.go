package llm

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// ParseSingleObject parses a plain JSON object from LLM output.
// Handles accidental language wrappers {"de":{...}} and truncated output.
func ParseSingleObject(logID, content string) (map[string]any, error) {
	log := logrus.WithField("log_id", logID)

	content = TrimToJSON(content)
	if content == "" {
		return nil, fmt.Errorf("no JSON object found in normalize response")
	}

	var obj map[string]any
	if err := json.Unmarshal([]byte(content), &obj); err == nil {
		for _, key := range []string{"de", "en", "es"} {
			if inner, ok := obj[key]; ok {
				if innerMap, ok := inner.(map[string]any); ok && len(obj) == 1 {
					log.Warnf("unwrapped accidental language wrapper %q", key)
					return innerMap, nil
				}
			}
		}
		return obj, nil
	}

	repaired := content
	for i := range 5 {
		repaired += "}"
		if err := json.Unmarshal([]byte(repaired), &obj); err == nil {
			log.Warnf("repaired truncated JSON by appending %d brace(s)", i+1)
			return obj, nil
		}
	}

	return nil, fmt.Errorf("unable to parse normalize response (content: %.1000s)", content)
}

// ExtractLangBlock finds the JSON object for a given language key inside content
// using brace counting — tolerates surrounding syntax errors.
func ExtractLangBlock(content, key string) map[string]any {
	search := `"` + key + `":`
	idx := strings.Index(content, search)
	if idx == -1 {
		return nil
	}
	rest := strings.TrimLeft(content[idx+len(search):], " \t\n\r")
	if !strings.HasPrefix(rest, "{") {
		return nil
	}
	depth := 0
	inString := false
	escaped := false
	for i, c := range rest {
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' && inString {
			escaped = true
			continue
		}
		if c == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				var obj map[string]any
				if err := json.Unmarshal([]byte(rest[:i+1]), &obj); err == nil {
					return obj
				}
				return nil
			}
		}
	}
	return nil
}

// TrimToJSON strips everything before the first '{'.
func TrimToJSON(content string) string {
	idx := strings.Index(content, "{")
	if idx == -1 {
		return ""
	}
	return content[idx:]
}
