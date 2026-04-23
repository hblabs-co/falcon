package helpers

import "go.mongodb.org/mongo-driver/v2/bson"

// defaultScoreThreshold mirrors falcon-match-engine's default. Kept in
// sync by convention — if you bump one, bump the other. Both read the
// same MATCH_ENGINE_SCORE_THRESHOLD env var at runtime, so a production
// rollout only needs to update the configmap.
const defaultScoreThreshold = float32(6.0)

// CurrentScoreThreshold returns the live score threshold from the
// environment. Every caller re-reads per invocation so a kubectl
// rollout-free configmap change takes effect on the next HTTP request
// or NATS event.
func CurrentScoreThreshold() float32 {
	return ParseFloat32("MATCH_ENGINE_SCORE_THRESHOLD", defaultScoreThreshold)
}

// VisibleMatchFilter is the canonical "what counts as a user-visible
// match" predicate. Used by:
//
//   - falcon-api /matches (what the app lists)
//   - falcon-signal (Live Activity total_matches count — must match the
//     list exactly, otherwise the Lock Screen reads higher than the app)
//
// Any future caller that surfaces match counts to users must go through
// here — if the definition of "visible" changes, changing it in one
// place flows through without divergence.
//
// Currently excludes:
//   - freelance.de listings (temporarily off, dispatch quality issue)
//   - matches whose LLM score is below MATCH_ENGINE_SCORE_THRESHOLD
//     (those stay in Mongo for analytics but aren't shown to the user
//     and aren't pushed to NATS match.result)
func VisibleMatchFilter(userID string) bson.M {
	return bson.M{
		"user_id":  userID,
		"platform": bson.M{"$ne": "freelance.de"},
		"score":    bson.M{"$gte": CurrentScoreThreshold()},
	}
}
