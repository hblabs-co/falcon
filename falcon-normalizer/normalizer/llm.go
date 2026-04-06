package normalizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
)

const userPromptTemplate = `Normalize the following project JSON into the structured format described in your instructions.
Respond ONLY with the JSON object {"en":{...},"de":{...},"es":{...}}. No markdown, no explanation.

{{project_json}}`

// llmResponse is the top-level object returned by the LLM.
type llmResponse struct {
	En map[string]any `json:"en"`
	De map[string]any `json:"de"`
	Es map[string]any `json:"es"`
}

type llmClient struct {
	http         *ownhttp.Client
	model        string
	provider     string
	systemPrompt string
}

func newLLMClient(systemPrompt string) (*llmClient, error) {
	values, err := helpers.ReadEnvs("LLM_URL", "LLM_API_KEY", "LLM_MODEL", "LLM_PROVIDER")
	if err != nil {
		return nil, err
	}
	url, key, model, provider := values[0], values[1], values[2], values[3]
	return &llmClient{
		http:         ownhttp.New(url, map[string]string{"Authorization": "Bearer " + key}),
		model:        model,
		provider:     provider,
		systemPrompt: systemPrompt,
	}, nil
}

// Normalize sends a PersistedProject to the LLM and returns the structured trilingual result
// along with the raw response content (useful for error reporting regardless of concurrency).
func (c *llmClient) Normalize(ctx context.Context, project *models.PersistedProject) (*llmResponse, string, error) {
	start := time.Now()
	log := logrus.WithField("project_id", project.ID)

	projectJSON, err := json.Marshal(project)
	if err != nil {
		return nil, "", fmt.Errorf("marshal project: %w", err)
	}

	userPrompt := strings.ReplaceAll(userPromptTemplate, "{{project_json}}", string(projectJSON))

	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := c.http.Post(ctx, "/v1/chat/completions", ownhttp.Request{
		Body: map[string]any{
			"model": c.model,
			"messages": []map[string]string{
				{"role": "system", "content": c.systemPrompt},
				{"role": "user", "content": userPrompt},
			},
			"temperature": 0.1,
			"max_tokens":  8192,
		},
		Result: &resp,
	}); err != nil {
		return nil, "", fmt.Errorf("llm request: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, "", fmt.Errorf("llm returned no choices")
	}

	content := resp.Choices[0].Message.Content

	// Write raw LLM response to file for debugging.
	// debugFile := fmt.Sprintf("/tmp/llm_response_%s.json", project.ID)
	// _ = os.WriteFile(debugFile, []byte(content), 0644)
	// log.Infof("LLM raw response written to %s", debugFile)

	raw, err := parseResponse(project.ID, content)
	if err != nil {
		return nil, content, fmt.Errorf("parse llm response: %w", err)
	}

	log.WithFields(logrus.Fields{
		"took":    time.Since(start).String(),
		"en_keys": len(raw.En),
		"de_keys": len(raw.De),
		"es_keys": len(raw.Es),
	}).Info("LLM normalized project")

	return raw, content, nil
}

// parseResponse handles three malformed LLM output patterns:
//  1. Perfect JSON  → direct unmarshal
//  2. Languages nested inside "en"  → extract de/es from en
//  3. Multiple top-level objects    → {"en":{...}}, {"de":{...}}, {"es":{...}}
//  4. Truncated JSON (missing })    → brace repair, then re-apply 1-3
func parseResponse(projectID, content string) (*llmResponse, error) {
	log := logrus.WithField("project_id", projectID)

	// Strip markdown fences and surrounding text.
	start := strings.Index(content, "{")
	if start == -1 {
		return nil, fmt.Errorf("no JSON object found (content: %.200s)", content)
	}
	content = content[start:]

	// Strategy 1 & 2: direct unmarshal (may succeed even with nested langs).
	var raw llmResponse
	if err := json.Unmarshal([]byte(content), &raw); err == nil {
		return fixNestedLanguages(log, &raw), nil
	}

	// Strategy 3: multiple top-level objects, e.g. {"en":{...}}, {"de":{...}}, {"es":{...}}
	if merged, mergeErr := mergeTopLevelObjects(log, content); mergeErr == nil {
		return fixNestedLanguages(log, merged), nil
	}

	// Strategy 4: truncated — repair by appending closing braces, then retry 1-3.
	repaired := content
	for i := range 5 {
		repaired += "}"
		if err := json.Unmarshal([]byte(repaired), &raw); err == nil {
			log.Warnf("repaired truncated LLM JSON by appending %d closing brace(s)", i+1)
			return fixNestedLanguages(log, &raw), nil
		}
		if merged, mergeErr := mergeTopLevelObjects(log, repaired); mergeErr == nil {
			log.Warnf("repaired truncated multi-object LLM JSON by appending %d closing brace(s)", i+1)
			return fixNestedLanguages(log, merged), nil
		}
	}

	return nil, fmt.Errorf("unable to parse after all repair strategies (content: %.300s)", content)
}

// fixNestedLanguages moves de/es out of en if the LLM nested them there.
func fixNestedLanguages(log *logrus.Entry, r *llmResponse) *llmResponse {
	if len(r.De) > 0 && len(r.Es) > 0 {
		return r // already correct
	}
	if len(r.En) == 0 {
		return r
	}

	moved := false
	if de, ok := r.En["de"]; ok {
		if deMap, ok := de.(map[string]any); ok {
			r.De = deMap
			delete(r.En, "de")
			moved = true
		}
	}
	if es, ok := r.En["es"]; ok {
		if esMap, ok := es.(map[string]any); ok {
			r.Es = esMap
			delete(r.En, "es")
			moved = true
		}
	}
	if moved {
		log.Warn("extracted de/es that were nested inside en")
	}
	return r
}

// mergeTopLevelObjects handles {"en":{...}}, {"de":{...}}, {"es":{...}} by
// decoding each object in sequence and merging keys into one llmResponse.
func mergeTopLevelObjects(log *logrus.Entry, content string) (*llmResponse, error) {
	dec := json.NewDecoder(strings.NewReader(content))
	merged := make(map[string]any)
	for dec.More() {
		var obj map[string]any
		if err := dec.Decode(&obj); err != nil {
			return nil, err
		}
		for k, v := range obj {
			merged[k] = v
		}
	}
	// Re-encode + decode into typed struct.
	b, err := json.Marshal(merged)
	if err != nil {
		return nil, err
	}
	var r llmResponse
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	log.Warn("merged multiple top-level JSON objects from LLM response")
	return &r, nil
}
