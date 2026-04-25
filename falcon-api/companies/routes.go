package companies

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// Routes implements server.RouteGroup. Exposes a single public endpoint
// that returns every company the platform knows about — used by the
// iOS client to pre-download logos into the App Group container so
// Live Activities and other widget-process surfaces can render them
// synchronously from disk (widgets can't do AsyncImage reliably; the
// snapshot is taken before any network call resolves).
//
// Public on purpose: the response is not personalised, contains no
// user data, and is the same for every caller. Keeping it auth-free
// means Live Activity-capable devices can still refresh the logo cache
// even if the session token is expired or not yet issued.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.GET("/companies", handleListCompanies)
}

// companyItem is the minimal shape the iOS client needs per company.
// We intentionally omit metadata, recruiter stats, and internal bson
// fields — callers only need enough to resolve "this company_logo_url
// maps to this logo file on disk". Anything more is dead weight in
// the 7-day cache refresh.
type companyItem struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	LogoURL  string `json:"logo_url"`
}

type listResponse struct {
	Companies []companyItem `json:"companies"`
	UpdatedAt time.Time     `json:"updated_at"`
	Count     int           `json:"count"`
}

func handleListCompanies(c *gin.Context) {
	ctx := c.Request.Context()

	// Return every known company. At current scale (hundreds, not
	// millions) this is a single round-trip — far cheaper than
	// paginating when clients refresh once a week.
	var docs []models.Company
	if err := system.GetStorage().GetMany(ctx, constants.MongoCompaniesCollection,
		bson.M{}, &docs); err != nil {
		logrus.Errorf("list companies: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list companies"})
		return
	}

	items := make([]companyItem, 0, len(docs))
	for _, d := range docs {
		items = append(items, companyItem{
			ID:       d.CompanyID,
			Name:     d.CompanyName,
			Platform: d.Source,
			LogoURL:  d.LogoMinioURL,
		})
	}

	c.JSON(http.StatusOK, listResponse{
		Companies: items,
		UpdatedAt: time.Now().UTC(),
		Count:     len(items),
	})
}
