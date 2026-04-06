package normalizer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
)

const normalizePromptTemplate = `Extract and normalize the following project JSON according to your instructions.
Respond ONLY with the JSON object (no language wrapper keys). No markdown, no explanation.

{{project_json}}`

const translatePromptTemplate = `Translate the human-readable text in the following normalized project JSON from German to {{target_language}}.
Respond ONLY with the translated JSON object directly (no language wrapper key). No markdown, no explanation.

{{de_json}}`

// llmResponse is the top-level object returned after both LLM steps.
type llmResponse struct {
	En map[string]any `json:"en"`
	De map[string]any `json:"de"`
	Es map[string]any `json:"es"`
}

type llmClient struct {
	http              *ownhttp.Client
	model             string
	provider          string
	normalizePrompt   string
	translatePrompt   string
}

func newLLMClient(normalizePrompt, translatePrompt string) (*llmClient, error) {
	values, err := helpers.ReadEnvs("LLM_URL", "LLM_API_KEY", "LLM_MODEL", "LLM_PROVIDER")
	if err != nil {
		return nil, err
	}
	url, key, model, provider := values[0], values[1], values[2], values[3]
	return &llmClient{
		http:            ownhttp.New(url, map[string]string{"Authorization": "Bearer " + key}),
		model:           model,
		provider:        provider,
		normalizePrompt: normalizePrompt,
		translatePrompt: translatePrompt,
	}, nil
}

// Normalize runs two LLM calls:
//  1. Extract and normalize the project into a structured German JSON.
//  2. Translate that German JSON into English and Spanish.
//
// Returns the trilingual result plus the raw LLM content from whichever step failed
// (useful for error recording).
func (c *llmClient) Normalize(ctx context.Context, project *models.PersistedProject) (*llmResponse, string, error) {
	log := logrus.WithField("project_id", project.ID)
	start := time.Now()

	// ── Step 1: extract → German ──────────────────────────────────────────────
	deContent, rawDE, err := c.normalizeDE(ctx, project)
	if err != nil {
		return nil, rawDE, err
	}
	log.Infof("step 1 done (de_keys=%d, took=%s)", len(deContent), time.Since(start))

	// ── Step 2: translate DE → EN + ES (parallel) ────────────────────────────
	step2Start := time.Now()
	en, es, rawTranslate, err := c.translateToEnEs(ctx, deContent)
	if err != nil {
		return nil, rawTranslate, err
	}
	log.Infof("step 2 done (en_keys=%d es_keys=%d, took=%s)", len(en), len(es), time.Since(step2Start))

	log.WithField("total", time.Since(start)).Info("project normalized (2-step)")
	return &llmResponse{En: en, De: deContent, Es: es}, rawDE, nil
}

// normalizeDE calls the LLM to produce a single structured JSON object in German.
func (c *llmClient) normalizeDE(ctx context.Context, project *models.PersistedProject) (map[string]any, string, error) {
	projectJSON, err := json.Marshal(project)
	if err != nil {
		return nil, "", fmt.Errorf("marshal project: %w", err)
	}

	userPrompt := strings.ReplaceAll(normalizePromptTemplate, "{{project_json}}", string(projectJSON))
	content, err := c.chat(ctx, c.normalizePrompt, userPrompt)
	if err != nil {
		return nil, "", fmt.Errorf("llm normalize request: %w", err)
	}

	result, parseErr := parseSingleObject(project.ID, content)
	if parseErr != nil {
		return nil, content, fmt.Errorf("parse normalize response: %w", parseErr)
	}
	return result, content, nil
}

// translateToEnEs fires two parallel LLM requests — one for EN, one for ES.
// Each request translates from the German object to a single target language,
// which reduces token count and eliminates cross-language hallucinations.
func (c *llmClient) translateToEnEs(ctx context.Context, de map[string]any) (map[string]any, map[string]any, string, error) {
	deJSON, err := json.Marshal(de)
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal de content: %w", err)
	}
	deStr := string(deJSON)

	type result struct {
		obj     map[string]any
		raw     string
		err     error
	}

	var wg sync.WaitGroup
	enCh := make(chan result, 1)
	esCh := make(chan result, 1)

	translate := func(targetLang, targetName string, ch chan<- result) {
		defer wg.Done()
		prompt := strings.ReplaceAll(translatePromptTemplate, "{{target_language}}", targetName)
		prompt = strings.ReplaceAll(prompt, "{{de_json}}", deStr)
		content, chatErr := c.chat(ctx, c.translatePrompt, prompt)
		if chatErr != nil {
			ch <- result{err: fmt.Errorf("llm translate %s request: %w", targetLang, chatErr), raw: content}
			return
		}
		obj, parseErr := parseSingleObject("translate-"+targetLang, content)
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
		logrus.Warnf("translate es failed (%v), falling back to en for es", esRes.err)
		return enRes.obj, enRes.obj, esRes.raw, nil
	}
	return enRes.obj, esRes.obj, "", nil
}

// chat sends a single chat completion request and returns the message content.
func (c *llmClient) chat(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
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

// parseSingleObject parses a plain JSON object (no language wrapper).
// Handles the LLM accidentally wrapping in {"de":{...}} or {"en":{...}}.
func parseSingleObject(projectID, content string) (map[string]any, error) {
	log := logrus.WithField("project_id", projectID)

	content = trimToJSON(content)
	if content == "" {
		return nil, fmt.Errorf("no JSON object found in normalize response")
	}

	// Try direct unmarshal.
	var obj map[string]any
	if err := json.Unmarshal([]byte(content), &obj); err == nil {
		// If the LLM accidentally wrapped in a language key, unwrap it.
		for _, key := range []string{"de", "en", "es"} {
			if inner, ok := obj[key]; ok {
				if innerMap, ok := inner.(map[string]any); ok && len(obj) == 1 {
					log.Warnf("unwrapped accidental language wrapper %q from normalize response", key)
					return innerMap, nil
				}
			}
		}
		return obj, nil
	}

	// Brace repair for truncated output.
	repaired := content
	for i := range 5 {
		repaired += "}"
		if err := json.Unmarshal([]byte(repaired), &obj); err == nil {
			log.Warnf("repaired truncated normalize JSON by appending %d brace(s)", i+1)
			return obj, nil
		}
	}

	return nil, fmt.Errorf("unable to parse normalize response (content: %.300s)", content)
}

// parseTranslateResponse parses {"en":{...},"es":{...}} from the translation step.
// Falls back to per-language brace extraction when the outer JSON is syntactically corrupt.
func parseTranslateResponse(content string) (map[string]any, map[string]any, error) {
	content = trimToJSON(content)
	if content == "" {
		return nil, nil, fmt.Errorf("no JSON object found in translate response")
	}

	// Strategy 1: direct unmarshal.
	var wrapper struct {
		En map[string]any `json:"en"`
		Es map[string]any `json:"es"`
	}
	if err := json.Unmarshal([]byte(content), &wrapper); err == nil {
		if len(wrapper.En) > 0 && len(wrapper.Es) > 0 {
			return wrapper.En, wrapper.Es, nil
		}
	}

	// Strategy 2: brace repair for truncated output.
	repaired := content
	for range 5 {
		repaired += "}"
		if err := json.Unmarshal([]byte(repaired), &wrapper); err == nil {
			if len(wrapper.En) > 0 {
				return wrapper.En, wrapper.Es, nil
			}
		}
	}

	// Strategy 3: the outer JSON may be corrupt (e.g. a syntax error inside one language block).
	// Extract each language block independently using brace counting.
	en := extractLangBlock(content, "en")
	es := extractLangBlock(content, "es")
	if en != nil {
		if es == nil {
			// en is valid but es is corrupt — use en as fallback for es rather than failing.
			logrus.Warn("parseTranslateResponse: es block corrupt, falling back to en for es")
			es = en
		}
		return en, es, nil
	}

	return nil, nil, fmt.Errorf("unable to parse translate response (content: %.300s)", content)
}

// extractLangBlock finds the JSON object for a given language key inside content
// using brace counting — tolerates surrounding syntax errors in other keys.
func extractLangBlock(content, key string) map[string]any {
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

// trimToJSON strips everything before the first '{'.
func trimToJSON(content string) string {
	idx := strings.Index(content, "{")
	if idx == -1 {
		return ""
	}
	return content[idx:]
}
