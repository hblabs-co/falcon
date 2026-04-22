package models

import "time"

// MatchPendingEvent is published to NATS subject "match.pending" by falcon-dispatch
// for every CV that exceeds the similarity threshold for a given project.
// falcon-match-engine workers consume this to produce a scored MatchResultEvent.
type MatchPendingEvent struct {
	CVID       string  `json:"cv_id"`
	QdrantID   string  `json:"qdrant_id"`
	UserID     string  `json:"user_id"`
	ProjectID  string  `json:"project_id"`
	Platform   string  `json:"platform"`
	Similarity float32 `json:"similarity"` // cosine score from Qdrant
}

// MatchScores holds the individual dimension scores (0–10) used to render
// the candidate card in the UI as progress bars.
type MatchScores struct {
	SkillsMatch        float32 `json:"skills_match"`
	SeniorityFit       float32 `json:"seniority_fit"`
	DomainExperience   float32 `json:"domain_experience"`
	CommunicationClarity float32 `json:"communication_clarity"`
	ProjectRelevance   float32 `json:"project_relevance"`
	TechStackOverlap   float32 `json:"tech_stack_overlap"`
}

// MatchLabel is the UI label shown on the candidate card.
type MatchLabel string

const (
	MatchLabelApplyImmediately MatchLabel = "apply_immediately" // score >= 8.5
	MatchLabelTopCandidate     MatchLabel = "top_candidate"     // score >= 7
	MatchLabelAcceptable       MatchLabel = "acceptable"        // score >= 5
	MatchLabelNotSuitable      MatchLabel = "not_suitable"      // score < 5
)

// MatchResultEvent is always written to MongoDB (match_results collection) regardless
// of score. Only events where PassedThreshold=true are published to NATS "match.result"
// and forwarded to falcon-signal. This allows full analytics over all LLM evaluations.
//
// Records are upserted by (cv_id, project_id) so re-scoring overwrites prior results.
type MatchResultEvent struct {
	CVID         string `json:"cv_id"        bson:"cv_id"`
	UserID       string `json:"user_id"      bson:"user_id"`
	ProjectID    string `json:"project_id"   bson:"project_id"`
	ProjectTitle string `json:"project_title" bson:"project_title"`
	Platform     string `json:"platform"     bson:"platform"`
	// CompanyName is the authoritative company name from the companies collection
	// (looked up by company_id+platform), not the LLM extraction. Empty if unknown.
	CompanyName string `json:"company_name" bson:"company_name"`

	// Overall score 0–10, average of the six dimension scores.
	Score  float32     `json:"score"  bson:"score"`
	Label  MatchLabel  `json:"label"  bson:"label"`
	Scores MatchScores `json:"scores" bson:"scores"`

	// Up to 5 skills that make the candidate a strong fit. Tech names — not translated.
	MatchedSkills []string `json:"matched_skills" bson:"matched_skills"`
	// Up to 5 skills missing from the CV that the project requires. Tech names — not translated.
	MissingSkills []string `json:"missing_skills" bson:"missing_skills"`

	// Sentences explaining the match, split by sentiment. Multi-language:
	// keys are "de", "en", "es". The German version is authoritative (LLM source);
	// EN/ES are translations. Falcon-signal reads the user's preferred language
	// for the push body; the iOS detail view picks the current app language.
	PositivePoints  map[string][]string `json:"positive_points"  bson:"positive_points"`
	NegativePoints  map[string][]string `json:"negative_points"  bson:"negative_points"`
	ImprovementTips map[string][]string `json:"improvement_tips" bson:"improvement_tips"`

	// PassedThreshold indicates whether this result was forwarded to NATS.
	// false means the score was below the configured threshold — stored for analytics only.
	PassedThreshold bool      `json:"passed_threshold" bson:"passed_threshold"`
	ScoredAt        time.Time `json:"scored_at"        bson:"scored_at"`

	// Summary is a one-sentence push notification body. Multi-language map.
	// Example DE: "Score 7.8 · React/TypeScript stark, fehlendes AWS und Docker."
	Summary map[string]string `json:"summary" bson:"summary"`

	// LLM metadata — identifies which model produced this score.
	LLMModel    string `json:"llm_model"    bson:"llm_model"`
	LLMProvider string `json:"llm_provider" bson:"llm_provider"`

	// Viewed flips to true when the user opens MatchDetailView for this
	// match. Drives the "unread dot" on cards and the tab-icon badge
	// count. Pre-existing rows without the field decode as false
	// (Go zero-value), so they're treated as unread until opened.
	Viewed bool `json:"viewed" bson:"viewed"`
}

// LabelFromScore derives the UI label from the overall score.
func LabelFromScore(score float32) MatchLabel {
	switch {
	case score >= 8.5:
		return MatchLabelApplyImmediately
	case score >= 7:
		return MatchLabelTopCandidate
	case score >= 5:
		return MatchLabelAcceptable
	default:
		return MatchLabelNotSuitable
	}
}
