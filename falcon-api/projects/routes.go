package projects

import (
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

const pageSize = 20

// Routes implements server.RouteGroup for project feed endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.GET("/projects", handleListProjects)
}

type projectItem struct {
	ProjectID            string                       `json:"project_id"`
	Platform             string                       `json:"platform"`
	PlatformUpdatedAt    string                       `json:"platform_updated_at"`
	CompanyName          string                       `json:"company_name"`
	CompanyLogoURL       string                       `json:"company_logo_url"`
	RecruiterRodeoStats  *models.RecruiterRodeoStats  `json:"recruiter_rodeo_stats,omitempty"`
	NormalizedAt         string                       `json:"normalized_at"`
	Data                 map[string]any               `json:"data"` // English normalized content
}

type paginationMeta struct {
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

// normalizedDoc is the minimal projection we decode from projects_normalized.
type normalizedDoc struct {
	ProjectID         string         `bson:"project_id"`
	Platform          string         `bson:"platform"`
	PlatformUpdatedAt string         `bson:"platform_updated_at"`
	CompanyName       string         `bson:"company_name"`
	En                map[string]any `bson:"en"`
	De                map[string]any `bson:"de"`
	Es                map[string]any `bson:"es"`
	NormalizedAt      time.Time      `bson:"normalized_at"`
}

// langContent returns the localized content map for the requested language,
// falling back through en → de → es → empty map so "data" is never null.
func (d *normalizedDoc) langContent(lang string) map[string]any {
	switch lang {
	case "de":
		if len(d.De) > 0 {
			return d.De
		}
	case "es":
		if len(d.Es) > 0 {
			return d.Es
		}
	}
	if len(d.En) > 0 {
		return d.En
	}
	if len(d.De) > 0 {
		return d.De
	}
	if len(d.Es) > 0 {
		return d.Es
	}
	return map[string]any{}
}

func handleListProjects(c *gin.Context) {
	page := 1
	if p := c.Query("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	lang := c.Query("lang")
	if lang != "de" && lang != "es" {
		lang = "en"
	}

	ctx := c.Request.Context()
	store := system.GetStorage()

	var docs []normalizedDoc
	total, err := store.FindPage(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{}, "platform_updated_at", true, page, pageSize, &docs)
	if err != nil {
		logrus.Errorf("list projects page=%d: %v", page, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch projects"})
		return
	}

	// Batch-fetch company logos by company name.
	nameSet := make(map[string]struct{}, len(docs))
	for _, d := range docs {
		if d.CompanyName != "" {
			nameSet[d.CompanyName] = struct{}{}
		}
	}
	logoMap := make(map[string]string, len(nameSet))
	statsMap := make(map[string]*models.RecruiterRodeoStats, len(nameSet))
	if len(nameSet) > 0 {
		names := make([]string, 0, len(nameSet))
		for n := range nameSet {
			names = append(names, n)
		}
		var companies []models.Company
		if err := store.GetManyByField(ctx, constants.MongoCompaniesCollection, "company_name", names, &companies); err != nil {
			logrus.Warnf("fetch company data: %v", err)
		}
		for _, co := range companies {
			logoMap[co.CompanyName] = co.LogoMinioURL
			if co.RecruiterRodeoStats != nil {
				statsMap[co.CompanyName] = co.RecruiterRodeoStats
			}
		}
	}

	items := make([]projectItem, len(docs))
	for i, d := range docs {
		items[i] = projectItem{
			ProjectID:           d.ProjectID,
			Platform:            d.Platform,
			PlatformUpdatedAt:   d.PlatformUpdatedAt,
			CompanyName:         d.CompanyName,
			CompanyLogoURL:      logoMap[d.CompanyName],
			RecruiterRodeoStats: statsMap[d.CompanyName],
			NormalizedAt:        d.NormalizedAt.Format(time.RFC3339),
			Data:                d.langContent(lang),
		}
	}

	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	c.JSON(http.StatusOK, gin.H{
		"data": items,
		"pagination": paginationMeta{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	})
}
