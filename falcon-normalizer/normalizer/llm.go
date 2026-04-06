package normalizer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

// Normalize sends a PersistedProject to the LLM and returns the structured trilingual result.
func (c *llmClient) Normalize(ctx context.Context, project *models.PersistedProject) (*llmResponse, error) {
	start := time.Now()

	projectJSON, err := json.Marshal(project)
	if err != nil {
		return nil, fmt.Errorf("marshal project: %w", err)
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
		return nil, fmt.Errorf("llm request: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm returned no choices")
	}

	content := resp.Choices[0].Message.Content

	// Write raw LLM response to file for debugging.
	debugFile := fmt.Sprintf("/tmp/llm_response_%s.json", project.ID)
	_ = os.WriteFile(debugFile, []byte(content), 0644)
	logrus.Infof("LLM raw response written to %s", debugFile)

	// Tolerate markdown fences or surrounding text.
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON object in llm response (content: %.200s)", content)
	}
	content = content[jsonStart : jsonEnd+1]

	var raw llmResponse
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		// LLM sometimes truncates the final closing brace(s). Try to repair.
		var repairErr error
		for i := range 5 {
			content += "}"
			if json.Unmarshal([]byte(content), &raw) == nil {
				logrus.Warnf("repaired truncated LLM JSON by appending %d closing brace(s)", i+1)
				repairErr = nil
				break
			}
			repairErr = err
		}
		if repairErr != nil {
			return nil, fmt.Errorf("parse llm response: %w (content: %.200s)", repairErr, content)
		}
	}

	logrus.WithFields(logrus.Fields{
		"project_id": project.ID,
		"took":       time.Since(start).String(),
	}).Info("LLM normalized project")

	return &raw, nil
}
