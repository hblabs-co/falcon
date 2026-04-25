package users

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// matchView is the full per-row payload for the matches tab. We
// keep most of the MatchResultEvent (skill arrays, dimension
// scores, multi-language summary + positive/negative/improvement
// sentences) so the row can render the rich breakdown; only LLM
// metadata + the `Normalized` flag drop off.
type matchView struct {
	CVID            string                 `json:"cv_id"`
	ProjectID       string                 `json:"project_id"`
	ProjectTitle    string                 `json:"project_title,omitempty"`
	Platform        string                 `json:"platform,omitempty"`
	CompanyName     string                 `json:"company_name,omitempty"`
	CompanyLogoURL  string                 `json:"company_logo_url,omitempty"`
	Score           float32                `json:"score"`
	Label           models.MatchLabel      `json:"label,omitempty"`
	Scores          models.MatchScores     `json:"scores"`
	MatchedSkills   []string               `json:"matched_skills,omitempty"`
	MissingSkills   []string               `json:"missing_skills,omitempty"`
	Summary         map[string]string      `json:"summary,omitempty"`
	PositivePoints  map[string][]string    `json:"positive_points,omitempty"`
	NegativePoints  map[string][]string    `json:"negative_points,omitempty"`
	ImprovementTips map[string][]string    `json:"improvement_tips,omitempty"`
	PassedThreshold bool                   `json:"passed_threshold"`
	Viewed          bool                   `json:"viewed,omitempty"`
	ScoredAt        time.Time              `json:"scored_at"`
}

// listUserMatches returns match_results for a user, paginated and
// sorted by scored_at desc.
//
// Query params:
//
//   - min_score (float)  → `score >= min_score` Mongo filter.
//   - viewed   ("false") → only rows where viewed != true (unread).
//
// Both filters are applied at the database layer so totals + pages
// track them accurately — flipping the toggle in the UI fetches a
// fresh page instead of just hiding rows on the client.
func listUserMatches(c *gin.Context) {
	id := c.Param("id")
	if _, ok := loadUserOr404(c, id); !ok {
		return
	}

	page := parseLimit(c.Query("page"), 1, 10000)
	pageSize := parseLimit(c.Query("page_size"), 20, 100)

	filter := bson.M{"user_id": id}
	if raw := c.Query("min_score"); raw != "" {
		if v, err := strconv.ParseFloat(raw, 32); err == nil && v > 0 {
			filter["score"] = bson.M{"$gte": float32(v)}
		}
	}
	// `$ne: true` covers both viewed=false and rows where the field
	// is missing entirely (legacy data, default Go zero-value).
	if c.Query("viewed") == "false" {
		filter["viewed"] = bson.M{"$ne": true}
	}

	var matches []models.MatchResultEvent
	total, err := system.GetStorage().FindPage(c.Request.Context(),
		constants.MongoMatchResultsCollection,
		filter, "scored_at", true, page, pageSize, &matches)
	if err != nil {
		respondInternal(c, "list user matches", err)
		return
	}

	out := make([]matchView, 0, len(matches))
	for _, m := range matches {
		out = append(out, toMatchView(m))
	}
	c.JSON(http.StatusOK, gin.H{
		"page":      page,
		"page_size": pageSize,
		"total":     total,
		"matches":   out,
	})
}

// toMatchView projects a stored MatchResultEvent into the wire
// shape the UI consumes — same fields the list and the detail
// endpoints both use.
func toMatchView(m models.MatchResultEvent) matchView {
	return matchView{
		CVID:            m.CVID,
		ProjectID:       m.ProjectID,
		ProjectTitle:    m.ProjectTitle,
		Platform:        m.Platform,
		CompanyName:     m.CompanyName,
		CompanyLogoURL:  m.CompanyLogoURL,
		Score:           m.Score,
		Label:           m.Label,
		Scores:          m.Scores,
		MatchedSkills:   m.MatchedSkills,
		MissingSkills:   m.MissingSkills,
		Summary:         m.Summary,
		PositivePoints:  m.PositivePoints,
		NegativePoints:  m.NegativePoints,
		ImprovementTips: m.ImprovementTips,
		PassedThreshold: m.PassedThreshold,
		Viewed:          m.Viewed,
		ScoredAt:        m.ScoredAt,
	}
}

// getMatchDetail returns the match document plus the related raw
// project and the normalised projects_normalized doc (whichever
// languages are present). Powers the /match drill-down page in
// nest, which adapts the iOS JobDetailView to a web layout.
func getMatchDetail(c *gin.Context) {
	cvID := c.Param("cv_id")
	projectID := c.Param("project_id")
	if cvID == "" || projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cv_id and project_id required"})
		return
	}
	ctx := c.Request.Context()

	var match models.MatchResultEvent
	if err := system.GetStorage().Get(ctx, constants.MongoMatchResultsCollection,
		bson.M{"cv_id": cvID, "project_id": projectID}, &match); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "match not found"})
		return
	}

	// Raw scraped project (best-effort — older matches may reference
	// projects that have aged out / been deleted; we still return
	// the match in that case).
	var rawProject map[string]any
	_ = system.GetStorage().Get(ctx, constants.MongoProjectsCollection,
		bson.M{"id": projectID}, &rawProject)

	// Normalised, language-keyed content (if normalizer has caught
	// up). Same best-effort policy.
	var normalized map[string]any
	_ = system.GetStorage().Get(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": projectID}, &normalized)

	c.JSON(http.StatusOK, gin.H{
		"match":      toMatchView(match),
		"project":    rawProject,
		"normalized": normalized,
	})
}
