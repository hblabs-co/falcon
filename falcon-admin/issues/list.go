package issues

import (
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// listIssues returns a unified, paginated list across the errors
// and warnings collections, sorted by occurred_at desc.
//
// Query params:
//
//	type     = "all" | "error" | "warning"   (default "all")
//	resolved = "true" | "false"              (default "false")
//	page     = 1-indexed page number         (default 1)
//	page_size= results per page              (default 20, max 100)
//	service  = optional service_name filter
//
// For type="all" we run the page query against each collection
// and merge in Go. The merge over-fetches a bit (page * pageSize
// per collection) so we always hand back a coherent slice without
// the cursor gymnastics a true union would require.
func listIssues(c *gin.Context) {
	ctx := c.Request.Context()

	typ := c.DefaultQuery("type", "all")
	if typ != "all" && typ != TypeError && typ != TypeWarning {
		typ = "all"
	}
	resolved := c.DefaultQuery("resolved", "false") == "true"
	page := parsePositive(c.Query("page"), 1, 10000)
	pageSize := parsePositive(c.Query("page_size"), 20, 100)
	service := c.Query("service")

	filter := bson.M{}
	// Default behaviour is "show only what's still open": resolved
	// rows hide unless the caller explicitly asks for them.
	if !resolved {
		filter["resolved"] = bson.M{"$ne": true}
	}
	if service != "" {
		filter["service_name"] = service
	}

	var (
		out         []issueView
		totalErrors int64
		totalWarns  int64
	)

	if typ == TypeError || typ == "all" {
		var errs []models.ServiceError
		total, err := system.GetStorage().FindPage(ctx,
			constants.MongoErrorsCollection, filter,
			"occurred_at", true, page, pageSize, &errs)
		if err != nil {
			respondInternal(c, "list errors", err)
			return
		}
		totalErrors = total
		for _, e := range errs {
			out = append(out, errorToView(e))
		}
	}
	if typ == TypeWarning || typ == "all" {
		var warns []models.ServiceWarning
		total, err := system.GetStorage().FindPage(ctx,
			constants.MongoWarningsCollection, filter,
			"occurred_at", true, page, pageSize, &warns)
		if err != nil {
			respondInternal(c, "list warnings", err)
			return
		}
		totalWarns = total
		for _, w := range warns {
			out = append(out, warningToView(w))
		}
	}

	// Sort merged slice by occurred_at desc (errors and warnings
	// were each sorted server-side, but interleaving by time
	// requires a final pass).
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].OccurredAt.After(out[j].OccurredAt)
	})
	if len(out) > pageSize {
		out = out[:pageSize]
	}

	total := totalErrors + totalWarns
	if typ == TypeError {
		total = totalErrors
	} else if typ == TypeWarning {
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
