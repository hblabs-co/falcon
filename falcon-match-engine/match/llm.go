package match

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

const systemPrompt = `You are a strict technical recruiter evaluating freelance candidates.
Your evaluations must be calibrated and honest — most candidates are NOT a perfect fit.

Scoring rules you must follow:
- Scores of 9 or 10 are reserved for exceptional, clearly demonstrated evidence. Do not award them by default.
- A score of 5 means average — the candidate partially meets the requirement but has notable gaps.
- If a must-have requirement from the project is clearly missing from the CV, the overall score cannot exceed 5.0.
- missing_skills must list every requirement not clearly evidenced in the CV. An empty array means the CV explicitly covers ALL requirements — this should be rare.
- negative_points must be honest. If there are gaps, list them. An empty array means there are zero concerns — use it sparingly.
- communication_clarity reflects only writing quality and structure, not technical strength. A score of 10 means flawless professional writing — rare.

Always respond with valid JSON only. No markdown, no explanation outside the JSON.`

const userPromptTemplate = `Evaluate the following CV against the project description. Be strict and calibrated.

Score each dimension from 0 to 10 using the full range — do not cluster scores in the 7-9 range:
- skills_match: how well the candidate's skills cover what the project needs (penalise hard for missing must-haves)
- seniority_fit: whether the experience level matches expectations (years, scope, complexity of past work)
- domain_experience: prior work in the same industry or problem domain
- communication_clarity: writing quality and CV structure only — not technical ability
- project_relevance: how similar past projects are in scope and type to this one
- tech_stack_overlap: literal overlap in frameworks, languages, and tools — inferred overlap does not count

Compute the overall score as the average of the six dimensions, rounded to one decimal.

Respond ONLY with this JSON (no markdown, no extra fields):
{
  "score": <average, one decimal>,
  "scores": {
    "skills_match": <0-10>,
    "seniority_fit": <0-10>,
    "domain_experience": <0-10>,
    "communication_clarity": <0-10>,
    "project_relevance": <0-10>,
    "tech_stack_overlap": <0-10>
  },
  "matched_skills": [<up to 5 skills the candidate clearly has AND the project needs — only explicit matches>],
  "missing_skills": [<up to 5 skills the project requires that are absent or unclear in the CV — be thorough>],
  "positive_points": [<2-4 honest sentences about genuine strengths for this specific project>],
  "negative_points": [<1-3 honest sentences about real gaps or concerns — required if score < 8>],
  "improvement_tips": [<up to 3 concrete, specific actions the candidate could take to improve chances on similar projects>]
}

PROJECT:
{{project_title}}

{{project_description}}

CV:
{{cv_text}}`

// llmResponse is the raw JSON the LLM returns.
type llmResponse struct {
	Score  float32 `json:"score"`
	Scores struct {
		SkillsMatch          float32 `json:"skills_match"`
		SeniorityFit         float32 `json:"seniority_fit"`
		DomainExperience     float32 `json:"domain_experience"`
		CommunicationClarity float32 `json:"communication_clarity"`
		ProjectRelevance     float32 `json:"project_relevance"`
		TechStackOverlap     float32 `json:"tech_stack_overlap"`
	} `json:"scores"`
	MatchedSkills   []string `json:"matched_skills"`
	MissingSkills   []string `json:"missing_skills"`
	PositivePoints  []string `json:"positive_points"`
	NegativePoints  []string `json:"negative_points"`
	ImprovementTips []string `json:"improvement_tips"`
}

// maxCVChars is the safety limit for CV text sent to the LLM (~12 000 tokens at ~4 chars/token).
const maxCVChars = 48_000

type llmClient struct {
	http  *ownhttp.Client
	model string
}

func newLLMClient() (*llmClient, error) {
	values, err := helpers.ReadEnvs("LLM_URL", "LLM_API_KEY", "LLM_MODEL")
	if err != nil {
		return nil, err
	}
	url, key, model := values[0], values[1], values[2]
	return &llmClient{
		http:  ownhttp.New(url, map[string]string{"Authorization": "Bearer " + key}),
		model: model,
	}, nil
}

// Score evaluates a CV against a project and returns the scored result.
func (c *llmClient) Score(ctx context.Context, projectTitle, projectDescription, cvText string) (*models.MatchResultEvent, error) {
	start := time.Now()

	if len(cvText) > maxCVChars {
		logrus.Warnf("CV text truncated from %d to %d chars before scoring", len(cvText), maxCVChars)
		cvText = cvText[:maxCVChars]
	}

	userPrompt := strings.NewReplacer(
		"{{project_title}}", projectTitle,
		"{{project_description}}", projectDescription,
		"{{cv_text}}", cvText,
	).Replace(userPromptTemplate)

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
		},
		Result: &resp,
	}); err != nil {
		return nil, fmt.Errorf("llm request: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("llm returned no choices")
	}

	content := resp.Choices[0].Message.Content

	// Extract the JSON object — tolerate markdown fences or surrounding text.
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		return nil, fmt.Errorf("no JSON object found in llm response (content: %.200s)", content)
	}
	content = content[jsonStart : jsonEnd+1]

	var raw llmResponse
	if err := json.Unmarshal([]byte(content), &raw); err != nil {
		return nil, fmt.Errorf("parse llm response: %w (content: %.200s)", err, content)
	}

	logrus.WithFields(logrus.Fields{
		"score": raw.Score,
		"took":  time.Since(start).String(),
	}).Info("LLM scored")

	result := &models.MatchResultEvent{
		Score: raw.Score,
		Label: models.LabelFromScore(raw.Score),
		Scores: models.MatchScores{
			SkillsMatch:          raw.Scores.SkillsMatch,
			SeniorityFit:         raw.Scores.SeniorityFit,
			DomainExperience:     raw.Scores.DomainExperience,
			CommunicationClarity: raw.Scores.CommunicationClarity,
			ProjectRelevance:     raw.Scores.ProjectRelevance,
			TechStackOverlap:     raw.Scores.TechStackOverlap,
		},
		MatchedSkills:   raw.MatchedSkills,
		MissingSkills:   raw.MissingSkills,
		PositivePoints:  raw.PositivePoints,
		NegativePoints:  raw.NegativePoints,
		ImprovementTips: raw.ImprovementTips,
	}

	return result, nil
}
