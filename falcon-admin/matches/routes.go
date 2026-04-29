// Package matches in falcon-admin owns the match-resource admin
// surface: today the drill-down endpoint at `/matches/:cv_id/:project_id`
// that powers Nest's `/match` page (raw + normalised + match for any
// cv/project pair). Per-user match listing lives in
// `falcon-admin/users/` because it's part of a user's view; the
// shared wire shape (`MatchView`, `ToMatchView`) is exported here so
// both endpoints render identical JSON to the UI.
//
// Mirrors `falcon-admin/auth/`, `falcon-admin/signal/`,
// `falcon-admin/users/`, `falcon-admin/issues/` — each admin domain
// owns its own Mount() so service.go stays a flat route map.
package matches

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// Mount wires the matches admin route(s) onto the already-
// authenticated admin group.
func Mount(parent *gin.RouterGroup) {
	matchesGroup := parent.Group("/matches")
	matchesGroup.GET("/:cv_id/:project_id", GetMatchDetail)
}

// MatchView is the wire shape for a single match row, used by both
// the per-user list and the drill-down. We keep most of the
// MatchResultEvent (skill arrays, dimension scores, multi-language
// summary + positive/negative/improvement sentences) so the row can
// render the rich breakdown; only LLM metadata + the `Normalized`
// flag drop off.
type MatchView struct {
	CVID            string              `json:"cv_id"`
	ProjectID       string              `json:"project_id"`
	ProjectTitle    string              `json:"project_title,omitempty"`
	Platform        string              `json:"platform,omitempty"`
	CompanyName     string              `json:"company_name,omitempty"`
	CompanyLogoURL  string              `json:"company_logo_url,omitempty"`
	Score           float32             `json:"score"`
	Label           models.MatchLabel   `json:"label,omitempty"`
	Scores          models.MatchScores  `json:"scores"`
	MatchedSkills   []string            `json:"matched_skills,omitempty"`
	MissingSkills   []string            `json:"missing_skills,omitempty"`
	Summary         map[string]string   `json:"summary,omitempty"`
	PositivePoints  map[string][]string `json:"positive_points,omitempty"`
	NegativePoints  map[string][]string `json:"negative_points,omitempty"`
	ImprovementTips map[string][]string `json:"improvement_tips,omitempty"`
	PassedThreshold bool                `json:"passed_threshold"`
	Viewed          bool                `json:"viewed,omitempty"`
	ScoredAt        time.Time           `json:"scored_at"`
}

// ToMatchView projects a stored MatchResultEvent into the wire shape
// the UI consumes — same fields the list and the detail endpoints
// both use.
func ToMatchView(m models.MatchResultEvent) MatchView {
	return MatchView{
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

// GetMatchDetail returns the match document plus the related raw
// project and the normalised projects_normalized doc (whichever
// languages are present). Powers the /match drill-down page in
// nest, which adapts the iOS JobDetailView to a web layout.
func GetMatchDetail(c *gin.Context) {
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
		"match":      ToMatchView(match),
		"project":    rawProject,
		"normalized": normalized,
	})
}
