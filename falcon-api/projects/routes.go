package projects

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

const pageSize = 20

// Routes implements server.RouteGroup for project feed endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.GET("/projects", handleListProjects)
	r.GET("/projects/:id", handleGetProject)
}

type projectItem struct {
	ProjectID           string                      `json:"project_id"`
	Platform            string                      `json:"platform"`
	CompanyName         string                      `json:"company_name"`
	CompanyLogoURL      string                      `json:"company_logo_url"`
	RecruiterRodeoStats *models.RecruiterRodeoStats `json:"recruiter_rodeo_stats,omitempty"`
	DisplayUpdatedAt    string                      `json:"display_updated_at"`
	NormalizedAt        string                      `json:"normalized_at"`
	Data                map[string]any              `json:"data"` // Normalized content in the requested language
}

// normalizedDoc is the minimal projection we decode from projects_normalized.
type normalizedDoc struct {
	ProjectID        string         `bson:"project_id"`
	CompanyID        string         `bson:"company_id"`
	DisplayUpdatedAt time.Time      `bson:"display_updated_at"`
	En               map[string]any `bson:"en"`
	De               map[string]any `bson:"de"`
	Es               map[string]any `bson:"es"`
	NormalizedAt     time.Time      `bson:"normalized_at"`
}

// nestedString walks a nested map path and returns the string at the end,
// or "" if any step is missing or the terminal value is not a string.
func nestedString(m map[string]any, keys ...string) string {
	cur := m
	for i, k := range keys {
		v, ok := cur[k]
		if !ok {
			return ""
		}
		if i == len(keys)-1 {
			s, _ := v.(string)
			return s
		}
		cur, ok = v.(map[string]any)
		if !ok {
			return ""
		}
	}
	return ""
}

// buildProjectItem composes the response shape from a normalized doc and an
// optional company (nil when unknown). Shared by the list and single endpoints
// so both return identical JSON for the same project.
func buildProjectItem(d *normalizedDoc, lang string, company *models.Company) projectItem {
	data := d.langContent(lang)
	item := projectItem{
		ProjectID:        d.ProjectID,
		Platform:         nestedString(data, "source", "platform"),
		DisplayUpdatedAt: d.DisplayUpdatedAt.Format(time.RFC3339),
		NormalizedAt:     d.NormalizedAt.Format(time.RFC3339),
		Data:             data,
	}
	if company != nil {
		item.Platform = company.Source
		item.CompanyName = company.CompanyName
		item.CompanyLogoURL = company.LogoMinioURL
		item.RecruiterRodeoStats = company.RecruiterRodeoStats
	}
	return item
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
	page := server.ParsePage(c)

	lang := c.Query("lang")
	if lang != "de" && lang != "es" {
		lang = "en"
	}

	ctx := c.Request.Context()
	store := system.GetStorage()

	// Filter out the "in_progress" claim placeholders the normalizer
	// writes on tryClaim before the LLM runs — those docs carry only
	// project_id + status + acquired_at and no en/de/es content, so
	// returning them would surface as empty cards in the iOS feed
	// (issue: "a card with just a random id and no content").
	listFilter := bson.M{"status": bson.M{"$ne": "in_progress"}}

	var docs []normalizedDoc
	total, err := store.FindPage(ctx, constants.MongoNormalizedProjectsCollection,
		listFilter, "display_updated_at", true, page, pageSize, &docs)
	if err != nil {
		logrus.Errorf("list projects page=%d: %v", page, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch projects"})
		return
	}

	// Batch-fetch companies by id so we can attach logo, stats, and name.
	idSet := make(map[string]struct{}, len(docs))
	for _, d := range docs {
		if d.CompanyID != "" {
			idSet[d.CompanyID] = struct{}{}
		}
	}
	companyMap := make(map[string]*models.Company, len(idSet))
	if len(idSet) > 0 {
		ids := make([]string, 0, len(idSet))
		for id := range idSet {
			ids = append(ids, id)
		}
		var companies []models.Company
		if err := store.GetManyByField(ctx, constants.MongoCompaniesCollection, "company_id", ids, &companies); err != nil {
			logrus.Warnf("fetch companies: %v", err)
		}
		for i := range companies {
			companyMap[companies[i].CompanyID] = &companies[i]
		}
	}

	items := make([]projectItem, len(docs))
	for i := range docs {
		items[i] = buildProjectItem(&docs[i], lang, companyMap[docs[i].CompanyID])
	}

	// Count projects normalized today (Berlin local day, not UTC — otherwise the
	// day rolls over at 2am Berlin in summer / 1am in winter, which surprises users).
	//
	// TODO: when the user base grows beyond Europe (LATAM/Asia), accept an optional
	// `?tz=` query param (e.g. tz=America/Bogota) and use that instead of Berlin.
	// Client (iOS) can send TimeZone.current.identifier. Default stays Berlin so
	// existing clients keep working unchanged.
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		berlin = time.UTC
	}
	nowBerlin := time.Now().In(berlin)
	startOfDay := time.Date(nowBerlin.Year(), nowBerlin.Month(), nowBerlin.Day(), 0, 0, 0, 0, berlin)
	todayCount, _ := store.Count(ctx, constants.MongoNormalizedProjectsCollection, bson.M{
		"normalized_at": bson.M{"$gte": startOfDay},
	})

	c.JSON(http.StatusOK, gin.H{
		"data":        items,
		"today_count": todayCount,
		"pagination":  server.Paginate(page, pageSize, total),
	})
}

// handleGetProject returns a single normalized project, formatted exactly like
// one item from the list endpoint so iOS can reuse JobDetailView unchanged.
func handleGetProject(c *gin.Context) {
	projectID := c.Param("id")
	if projectID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id is required"})
		return
	}

	lang := c.Query("lang")
	if lang != "de" && lang != "es" {
		lang = "en"
	}

	ctx := c.Request.Context()
	store := system.GetStorage()

	var doc normalizedDoc
	if err := store.Get(ctx, constants.MongoNormalizedProjectsCollection,
		bson.M{"project_id": projectID}, &doc); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
		return
	}

	var companyPtr *models.Company
	if doc.CompanyID != "" {
		var company models.Company
		if err := store.Get(ctx, constants.MongoCompaniesCollection,
			bson.M{"company_id": doc.CompanyID}, &company); err == nil && company.CompanyID != "" {
			companyPtr = &company
		}
	}

	c.JSON(http.StatusOK, buildProjectItem(&doc, lang, companyPtr))
}
