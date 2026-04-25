package models

// RecruiterRodeoStats holds ratings scraped from the Recruiter Rodeo platform
// (uplink.tech/recruiters) and is embedded in the Company document.
type RecruiterRodeoStats struct {
	OverallRating      float64 `json:"overall_rating"      bson:"overall_rating"`      // e.g. 2.7
	RecommendationRate string  `json:"recommendation_rate" bson:"recommendation_rate"` // e.g. "55%"
	ReviewCount        int     `json:"review_count"        bson:"review_count"`        // e.g. 40
}
