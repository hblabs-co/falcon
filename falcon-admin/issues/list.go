package issues

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// listIssues returns a paginated list of errors OR warnings, sorted
// by occurred_at desc. The unified "all" tab from earlier was dropped
// in the UI — operators triage one bucket at a time — but the
// response still includes BOTH total_errors and total_warns so the
// other tab's count badge can update without an extra round-trip.
//
// Query params:
//
//	type     = "error" | "warning"           (default "error")
//	resolved = "true" | "false"              (default "false")
//	page     = 1-indexed page number         (default 1)
//	page_size= results per page              (default 20, max 100)
//	service  = optional service_name filter
//
// freelance.de is filtered out unconditionally: operators flagged it
// as a known-noisy platform whose errors and warnings are always
// expected. Re-enable by removing the explicit exclusion below.
func listIssues(c *gin.Context) {
	ctx := c.Request.Context()

	typ := c.DefaultQuery("type", TypeError)
	if typ != TypeError && typ != TypeWarning {
		typ = TypeError
	}
	resolved := c.DefaultQuery("resolved", "false") == "true"
	page := parsePositive(c.Query("page"), 1, 10000)
	pageSize := parsePositive(c.Query("page_size"), 20, 100)
	service := c.Query("service")

	// Shared "scope" filter — applied to both the page query and the
	// counts so the badge numbers always match what the operator
	// would see if they switched tabs.
	scope := bson.M{
		"platform": bson.M{"$ne": "freelance.de"},
	}
	if !resolved {
		scope["resolved"] = bson.M{"$ne": true}
	}
	if service != "" {
		scope["service_name"] = service
	}

	// Counts for BOTH collections, regardless of the type being paged.
	// Cheap with the resolved/platform indexes; lets the UI update the
	// other tab's badge from the same payload.
	totalErrors, err := system.GetStorage().Count(ctx, constants.MongoErrorsCollection, scope)
	if err != nil {
		respondInternal(c, "count errors", err)
		return
	}
	totalWarns, err := system.GetStorage().Count(ctx, constants.MongoWarningsCollection, scope)
	if err != nil {
		respondInternal(c, "count warnings", err)
		return
	}

	var out []issueView
	switch typ {
	case TypeError:
		var errs []models.ServiceError
		if _, err := system.GetStorage().FindPage(ctx,
			constants.MongoErrorsCollection, scope,
			"occurred_at", true, page, pageSize, &errs); err != nil {
			respondInternal(c, "list errors", err)
			return
		}
		for _, e := range errs {
			out = append(out, errorToView(e))
		}
	case TypeWarning:
		var warns []models.ServiceWarning
		if _, err := system.GetStorage().FindPage(ctx,
			constants.MongoWarningsCollection, scope,
			"occurred_at", true, page, pageSize, &warns); err != nil {
			respondInternal(c, "list warnings", err)
			return
		}
		for _, w := range warns {
			out = append(out, warningToView(w))
		}
	}

	total := totalErrors
	if typ == TypeWarning {
		total = totalWarns
	}

	c.JSON(http.StatusOK, gin.H{
		"page":         page,
		"page_size":    pageSize,
		"total":        total,
		"total_errors": totalErrors,
		"total_warns":  totalWarns,
		"issues":       out,
	})
}

func errorToView(e models.ServiceError) issueView {
	return issueView{
		Type:            TypeError,
		ID:              e.ID,
		Service:         e.ServiceName,
		Name:            e.ErrorName,
		Message:         e.Error,
		Priority:        string(e.Priority),
		Resolved:        e.Resolved,
		OccurredAt:      e.OccurredAt,
		LastSeenAt:      e.LastSeenAt,
		OccurrenceCount: e.OccurrenceCount,
		Platform:        e.Platform,
		PlatformID:      e.PlatformID,
		URL:             e.URL,
		ProjectID:       e.ProjectID,
		UserID:          e.UserID,
		CVID:            e.CVID,
		RetryCount:      e.RetryCount,
		// Heavy details — the UI shows them in an expandable panel
		// that's collapsed by default, so the per-row weight only
		// matters when the operator clicks "details". Worth the
		// payload size to avoid a second round-trip per row.
		StackTrace: e.StackTrace,
		HTML:       e.HTML,
	}
}

func warningToView(w models.ServiceWarning) issueView {
	return issueView{
		Type:       TypeWarning,
		ID:         w.ID,
		Service:    w.ServiceName,
		Name:       w.WarningName,
		Message:    w.Message,
		Priority:   string(w.Priority),
		Resolved:   w.Resolved,
		OccurredAt: w.OccurredAt,
		Platform:   w.Platform,
		// Warnings carry an HTML snapshot instead of a stack trace —
		// scout uses it to capture markup state at the moment a
		// drift warning fires. Same expandable-panel treatment as
		// the error stack trace.
		HTML: w.HTML,
	}
}

func parsePositive(raw string, def, max int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func respondInternal(c *gin.Context, what string, err error) {
	logrus.Errorf("[admin] %s: %v", what, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}
