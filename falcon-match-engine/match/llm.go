package match

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/llm"
	"hblabs.co/falcon/packages/models"
)

const scoreSystemPrompt = `You are a strict technical recruiter evaluating freelance candidates.
Your evaluations must be calibrated and honest — most candidates are NOT a perfect fit.

Scoring rules you must follow:
- Scores of 9 or 10 are reserved for exceptional, clearly demonstrated evidence. Do not award them by default.
- A score of 5 means average — the candidate partially meets the requirement but has notable gaps.
- If a must-have requirement from the project is clearly missing from the CV, the overall score cannot exceed 5.0.
- missing_skills must list every requirement not clearly evidenced in the CV. An empty array means the CV explicitly covers ALL requirements — this should be rare.
- negative_points must be honest. If there are gaps, list them. An empty array means there are zero concerns — use it sparingly.
- communication_clarity reflects only writing quality and structure, not technical strength. A score of 10 means flawless professional writing — rare.
- summary must be a single sentence in German, max 120 characters, suitable as an iOS push notification body. Format: "Score {score} · {strongest fit reason}, {main gap if any}."

All text fields in the JSON response (positive_points, negative_points, improvement_tips, summary) must be written in German.
matched_skills and missing_skills contain technology/skill names in their original form (not translated).

Always respond with valid JSON only. No markdown, no explanation outside the JSON.`

const scoreUserPromptTemplate = `Evaluate the following CV against the project description. Be strict and calibrated.

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
  "improvement_tips": [<up to 3 concrete, specific actions the candidate could take to improve chances on similar projects>],
  "summary": "<single sentence in German, max 120 chars, for iOS push notification>",
  "project_title": "<cleaned project title — strip platform ID prefixes like 'Projekt-Nr: 62737 - ', 'Job-Nr. 12345 - ', 'Ref: ABC-123 - ', '#62737 - '; strip gender suffixes '(m/w/d)', '(w/m/d)', '(d/m/w)'; remove trailing location if it repeats the location field; keep the core role + key tech stack; stay in German>"
}

PROJECT:
{{project_title}}

{{project_description}}

CV:
{{cv_text}}`

const translateSystemPrompt = `You are a translation engine for structured match-result JSON.

You receive a JSON object where the human-readable text fields are in German, and you return a copy where those fields are translated into the requested target language.

Output rules:
- Respond ONLY with a valid JSON object. No markdown, no explanation.
- Preserve the exact structure and every key name without exception.
- Translate ONLY the strings inside: summary, positive_points, negative_points, improvement_tips.
- Keep identical (do not translate): dates, numbers, booleans, null, URLs, snake_case identifiers, ISO codes, and all entries in matched_skills and missing_skills (those are tech names like "React", "AWS", "Kubernetes").
- If a string is already in the target language or is a proper noun/tech term, keep it as-is.`

// maxCVChars is the safety limit for CV text sent to the LLM (~12 000 tokens at ~4 chars/token).
const maxCVChars = 48_000

// llmRaw is the raw JSON shape the scoring LLM returns.
type llmRaw struct {
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
	Summary         string   `json:"summary"`
	// Clean title stripped of platform job-number prefixes and gender
	// suffixes. Provides a fallback when the normalizer hasn't processed
	// this project yet (race condition): service.go chains
	//   normalized.display → raw stripped → project.Title.
	ProjectTitle string `json:"project_title"`
}

// scorer wraps the shared LLM client with match-specific prompts and a translate
// step so each match_result carries de/en/es text fields.
type scorer struct {
	llm *llm.Client
}

func newScorer(client *llm.Client) *scorer {
	return &scorer{llm: client}
}

// Score evaluates a CV against a project, translates the human-readable output
// to EN+ES, and returns a MatchResultEvent ready to persist (caller fills in
// identifiers, timestamps, and threshold flag).
//
// Returns the raw LLM content alongside any error for diagnostics.
func (s *scorer) Score(
	ctx context.Context,
	projectTitle, projectDescription, cvText string,
	logFields map[string]any,
) (*models.MatchResultEvent, string, error) {
	start := time.Now()

	if len(cvText) > maxCVChars {
		logrus.WithFields(logFields).Warnf("CV text truncated from %d to %d chars before scoring", len(cvText), maxCVChars)
		cvText = cvText[:maxCVChars]
	}

	userPrompt := strings.NewReplacer(
		"{{project_title}}", projectTitle,
		"{{project_description}}", projectDescription,
		"{{cv_text}}", cvText,
	).Replace(scoreUserPromptTemplate)

	content, err := s.llm.Chat(ctx, scoreSystemPrompt, userPrompt)
	if err != nil {
		return nil, content, fmt.Errorf("llm score request: %w", err)
	}

	deJSON, err := llm.ParseSingleObject(fmt.Sprintf("%v", logFields["project_id"]), content)
	if err != nil {
		return nil, content, fmt.Errorf("parse score response: %w", err)
	}

	// Re-encode to typed struct for downstream mapping.
	deBytes, _ := json.Marshal(deJSON)
	var raw llmRaw
	if err := json.Unmarshal(deBytes, &raw); err != nil {
		return nil, content, fmt.Errorf("decode score response: %w", err)
	}

	// Translate the German response to EN+ES in parallel. Failures fall back to
	// the German text for that language — better degraded than empty.
	enTexts, esTexts := s.translate(ctx, deBytes, logFields)

	result := &models.MatchResultEvent{
		Score:       raw.Score,
		Label:       models.LabelFromScore(raw.Score),
		LLMModel:    s.llm.Model,
		LLMProvider: s.llm.Provider,
		// LLM-cleaned title — used as fallback when the normalizer hasn't
		// produced projects_normalized.<lang>.title.display yet. service.go
		// resolves the final title via normalized → this → raw chain.
		ProjectTitle: raw.ProjectTitle,
		Scores: models.MatchScores{
			SkillsMatch:          raw.Scores.SkillsMatch,
			SeniorityFit:         raw.Scores.SeniorityFit,
			DomainExperience:     raw.Scores.DomainExperience,
			CommunicationClarity: raw.Scores.CommunicationClarity,
			ProjectRelevance:     raw.Scores.ProjectRelevance,
			TechStackOverlap:     raw.Scores.TechStackOverlap,
		},
		MatchedSkills: raw.MatchedSkills,
		MissingSkills: raw.MissingSkills,
		Summary: map[string]string{
			"de": raw.Summary,
			"en": enTexts.Summary,
			"es": esTexts.Summary,
		},
		PositivePoints: map[string][]string{
			"de": raw.PositivePoints,
			"en": enTexts.PositivePoints,
			"es": esTexts.PositivePoints,
		},
		NegativePoints: map[string][]string{
			"de": raw.NegativePoints,
			"en": enTexts.NegativePoints,
			"es": esTexts.NegativePoints,
		},
		ImprovementTips: map[string][]string{
			"de": raw.ImprovementTips,
			"en": enTexts.ImprovementTips,
			"es": esTexts.ImprovementTips,
		},
	}

	logrus.WithFields(logFields).WithFields(logrus.Fields{
		"score": raw.Score,
		"took":  time.Since(start).String(),
	}).Info("LLM scored + translated")

	return result, content, nil
}

// translatedTexts holds just the human-readable fields that are language-specific.
type translatedTexts struct {
	Summary         string
	PositivePoints  []string
	NegativePoints  []string
	ImprovementTips []string
}

// translate runs EN and ES translations in parallel. On error for a given
// language, falls back to the German source to keep the record consistent.
func (s *scorer) translate(ctx context.Context, deJSON []byte, logFields map[string]any) (en, es translatedTexts) {
	var deFallback llmRaw
	_ = json.Unmarshal(deJSON, &deFallback)

	fallback := translatedTexts{
		Summary:         deFallback.Summary,
		PositivePoints:  deFallback.PositivePoints,
		NegativePoints:  deFallback.NegativePoints,
		ImprovementTips: deFallback.ImprovementTips,
	}

	var wg sync.WaitGroup
	var enOut, esOut translatedTexts

	runOne := func(targetLang, targetName string, out *translatedTexts) {
		defer wg.Done()
		translated, err := s.translateOne(ctx, deJSON, targetLang, targetName, logFields)
		if err != nil {
			logrus.WithFields(logFields).Warnf("translate %s failed (%v), falling back to de", targetLang, err)
			*out = fallback
			return
		}
		*out = translated
	}

	wg.Add(2)
	go runOne("en", "English", &enOut)
	go runOne("es", "Spanish", &esOut)
	wg.Wait()

	return enOut, esOut
}

func (s *scorer) translateOne(
	ctx context.Context,
	deJSON []byte,
	targetLang, targetName string,
	logFields map[string]any,
) (translatedTexts, error) {
	userPrompt := fmt.Sprintf(
		"Translate the human-readable strings in this match-result JSON from German to %s. Return only the translated JSON object.\n\n%s",
		targetName, string(deJSON))

	content, err := s.llm.Chat(ctx, translateSystemPrompt, userPrompt)
	if err != nil {
		return translatedTexts{}, fmt.Errorf("llm translate request: %w", err)
	}

	obj, err := llm.ParseSingleObject(
		fmt.Sprintf("%v-translate-%s", logFields["project_id"], targetLang),
		content,
	)
	if err != nil {
		return translatedTexts{}, fmt.Errorf("parse translate response: %w", err)
	}

	// Re-encode to typed struct.
	b, _ := json.Marshal(obj)
	var raw llmRaw
	if err := json.Unmarshal(b, &raw); err != nil {
		return translatedTexts{}, fmt.Errorf("decode translate response: %w", err)
	}

	return translatedTexts{
		Summary:         raw.Summary,
		PositivePoints:  raw.PositivePoints,
		NegativePoints:  raw.NegativePoints,
		ImprovementTips: raw.ImprovementTips,
	}, nil
}
