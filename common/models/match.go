package models

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

// MatchResultEvent is published to NATS subject "match.result" by falcon-match-engine
// once the LLM has scored a CV/project pair above the configured threshold.
// falcon-signal consumes this to notify the candidate.
type MatchResultEvent struct {
	CVID      string  `json:"cv_id"`
	UserID    string  `json:"user_id"`
	ProjectID string  `json:"project_id"`
	Platform  string  `json:"platform"`

	// Overall score 0–10, average of the six dimension scores.
	Score  float32    `json:"score"`
	Label  MatchLabel `json:"label"`
	Scores MatchScores `json:"scores"`

	// Up to 5 skills that make the candidate a strong fit.
	MatchedSkills []string `json:"matched_skills"`
	// Up to 5 skills missing from the CV that the project requires.
	MissingSkills []string `json:"missing_skills"`

	// Sentences explaining the match, split by sentiment for UI rendering.
	PositivePoints []string `json:"positive_points"`
	NegativePoints []string `json:"negative_points"`

	// Up to 3 actionable tips the candidate could add to their CV to improve
	// their chances on similar projects.
	ImprovementTips []string `json:"improvement_tips"`
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
