package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/ownhttp"
)

// Response is the trilingual result from a 2-step normalize + translate pipeline.
type Response struct {
	En map[string]any `json:"en"`
	De map[string]any `json:"de"`
	Es map[string]any `json:"es"`
}

// Client wraps LLM API communication. It exposes generic primitives —
// callers provide system prompts and user prompts specific to their domain.
type Client struct {
	http     *ownhttp.Client
	Model    string
	Provider string
	// TranslatePrompt is the system prompt used for DE→EN/ES translation.
	// Shared across all modules since the translation step is identical.
	TranslatePrompt string
}

// NewFromEnv creates a Client from LLM_URL, LLM_API_KEY, LLM_MODEL, LLM_PROVIDER env vars.
func NewFromEnv(translatePrompt string) (*Client, error) {
	values, err := helpers.ReadEnvs("LLM_URL", "LLM_API_KEY", "LLM_MODEL", "LLM_PROVIDER")
	if err != nil {
		return nil, err
	}
	url, key, model, provider := values[0], values[1], values[2], values[3]
	return &Client{
		http:            ownhttp.New(url, map[string]string{"Authorization": "Bearer " + key}),
		Model:           model,
		Provider:        provider,
		TranslatePrompt: translatePrompt,
	}, nil
}

// NormalizeDE calls the LLM with a system prompt and user prompt, and parses the
// result as a single JSON object. Returns the parsed map and the raw LLM content.
func (c *Client) NormalizeDE(ctx context.Context, systemPrompt, userPrompt, logID string) (map[string]any, string, error) {
	content, err := c.Chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, "", fmt.Errorf("llm normalize request: %w", err)
	}
	result, parseErr := ParseSingleObject(logID, content)
	if parseErr != nil {
		return nil, content, fmt.Errorf("parse normalize response: %w", parseErr)
	}
	return result, content, nil
}

// TranslateToEnEs fires two parallel LLM requests to translate a German JSON
// object into English and Spanish using the shared TranslatePrompt.
// logFields attaches caller context (e.g. project_id, cv_id) to any warnings emitted here.
func (c *Client) TranslateToEnEs(ctx context.Context, de map[string]any, logFields map[string]any) (en, es map[string]any, rawContent string, err error) {
	deJSON, marshalErr := json.Marshal(de)
	if marshalErr != nil {
		return nil, nil, "", fmt.Errorf("marshal de content: %w", marshalErr)
	}
	deStr := string(deJSON)

	translatePromptTemplate := `Translate the human-readable text in the following normalized project JSON from German to {{target_language}}.
Respond ONLY with the translated JSON object directly (no language wrapper key). No markdown, no explanation.

{{de_json}}`

	type result struct {
		obj map[string]any
		raw string
		err error
	}

	var wg sync.WaitGroup
	enCh := make(chan result, 1)
	esCh := make(chan result, 1)

	translate := func(targetLang, targetName string, ch chan<- result) {
		defer wg.Done()
		prompt := strings.ReplaceAll(translatePromptTemplate, "{{target_language}}", targetName)
		prompt = strings.ReplaceAll(prompt, "{{de_json}}", deStr)
		content, chatErr := c.Chat(ctx, c.TranslatePrompt, prompt)
		if chatErr != nil {
			ch <- result{err: fmt.Errorf("llm translate %s request: %w", targetLang, chatErr), raw: content}
			return
		}
		obj, parseErr := ParseSingleObject("translate-"+targetLang, content)
		if parseErr != nil {
			ch <- result{err: fmt.Errorf("parse translate %s response: %w", targetLang, parseErr), raw: content}
			return
		}
		ch <- result{obj: obj, raw: content}
	}

	wg.Add(2)
	go translate("en", "English", enCh)
	go translate("es", "Spanish", esCh)
	wg.Wait()

	enRes := <-enCh
	esRes := <-esCh

	if enRes.err != nil {
		return nil, nil, enRes.raw, enRes.err
	}
	if esRes.err != nil {
		logrus.WithFields(logFields).Warnf("translate es failed (%v), falling back to en for es", esRes.err)
		return enRes.obj, enRes.obj, esRes.raw, nil
	}
	return enRes.obj, esRes.obj, "", nil
}

// Chat sends a single chat completion request and returns the message content.
func (c *Client) Chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	var resp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := c.http.Post(ctx, "/v1/chat/completions", ownhttp.Request{
		Body: map[string]any{
			"model": c.Model,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userPrompt},
			},
			"temperature": 0.1,
			"max_tokens":  8192,
		},
		Result: &resp,
	}); err != nil {
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return resp.Choices[0].Message.Content, nil
}
