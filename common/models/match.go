package models

// MatchPending is published to NATS subject "match.pending" by falcon-dispatch
// for every CV that exceeds the similarity threshold for a given project.
// falcon-match-engine workers consume this to produce a scored MatchResult.
type MatchPending struct {
	CVID       string  `json:"cv_id"`
	QdrantID   string  `json:"qdrant_id"`
	UserID     string  `json:"user_id"`
	ProjectID  string  `json:"project_id"`
	Platform   string  `json:"platform"`
	Similarity float32 `json:"similarity"` // cosine score from Qdrant
}

// MatchResult is published to NATS subject "match.result" by falcon-match-engine
// once the LLM has scored a CV/project pair above the configured threshold.
// falcon-signal consumes this to notify the candidate.
type MatchResult struct {
	CVID        string  `json:"cv_id"`
	UserID      string  `json:"user_id"`
	ProjectID   string  `json:"project_id"`
	Platform    string  `json:"platform"`
	Score       float32 `json:"score"`       // LLM score 0–10
	Explanation string  `json:"explanation"` // human-readable, shown to the candidate
}
