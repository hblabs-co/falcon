package freelancede

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
	"hblabs.co/falcon/common/system"
)

const candidatesPageSize = 25

// shouldContinue decides whether to fetch the next page.
type shouldContinue func(candidates []*ProjectCandidate, newOnPage, totalOnPage int) bool

// stopOnOverlap stops when the page has some already-known candidates (normal polling).
func stopOnOverlap(candidates []*ProjectCandidate, newOnPage, totalOnPage int) bool {
	return newOnPage >= totalOnPage && totalOnPage >= candidatesPageSize
}

// stopOnYesterday stops only when any candidate on the page is from before today.
// Ignores page size — keeps going even on partial pages as long as all items are from today.
func stopOnYesterday(candidates []*ProjectCandidate, _, _ int) bool {
	today := time.Now().Format("2006-01-02")
	for _, c := range candidates {
		candidateDate := extractDate(c.PlatformUpdatedAt)
		if candidateDate != today {
			getLogger().Debugf("[stopOnYesterday] hit non-today candidate: platform_id=%s date=%q parsed=%s today=%s", c.PlatformID, c.PlatformUpdatedAt, candidateDate, today)
			return false
		}
	}
	return true
}

var dateFormats = []string{
	time.RFC3339,
	"2006-01-02T15:04:05Z",
	"02.01.2006 15:04",
	"02.01.2006",
}

func extractDate(s string) string {
	for _, format := range dateFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

// collectNewCandidates uses overlap detection to collect only recent candidates (normal polling).
func collectNewCandidates(ctx context.Context) ([]*ProjectCandidate, error) {
	return collectCandidates(ctx, "collectNewCandidates", stopOnOverlap)
}

// collectTodayCandidates pages through all of today's candidates (admin-triggered full scan).
func collectTodayCandidates(ctx context.Context) ([]*ProjectCandidate, error) {
	return collectCandidates(ctx, "collectTodayCandidates", stopOnYesterday)
}

// collectCandidates is the shared pagination loop. The shouldContinue callback
// controls when to stop fetching the next page.
func collectCandidates(ctx context.Context, name string, cont shouldContinue) ([]*ProjectCandidate, error) {
	s := getSession()
	token, err := s.AccessToken(ctx)
	if err != nil {
		return nil, err
	}

	var all []*ProjectCandidate

	for page := 1; ; page++ {
		candidates, err := fetchCandidatesPage(ctx, s, token, page)
		if err != nil {
			return nil, err
		}
		n := len(candidates)
		if n == 0 {
			break
		}

		filtered, err := filterCandidates(ctx, candidates)
		if err != nil {
			return nil, err
		}

		all = append(all, filtered...)
		shouldCont := cont(candidates, len(filtered), n)
		getLogger().Debugf("[%s] page %d: %d/%d new, continue=%v", name, page, len(filtered), n, shouldCont)

		if !shouldCont {
			break
		}
	}

	return all, nil
}

// filterCandidates removes candidates that are already up-to-date or have pending errors.
func filterCandidates(ctx context.Context, candidates []*ProjectCandidate) ([]*ProjectCandidate, error) {
	n := len(candidates)
	platformIDs := make([]string, n)
	for i, c := range candidates {
		platformIDs[i] = c.PlatformID
	}

	// Check which candidates already exist in the projects collection.
	var existing []models.PersistedProject
	if err := system.GetStorage().GetManyByField(ctx, constants.MongoProjectsCollection, "platform_id", platformIDs, &existing); err != nil {
		return nil, err
	}
	existingMap := make(map[string]models.PersistedProject, len(existing))
	for _, p := range existing {
		existingMap[p.PlatformID] = p
	}

	// Check which candidates have pending scrape errors (retry worker handles those).
	var pendingErrors []models.ServiceError
	if err := system.GetStorage().GetMany(ctx, constants.MongoErrorsCollection, bson.M{
		"service_name": constants.ServiceScout,
		"platform":     Source,
		"platform_id":  bson.M{"$in": platformIDs},
		"error_name": bson.M{"$in": []string{
			constants.ErrNameScrapeInspectFailed,
			constants.ErrNameScrapeServerError,
		}},
	}, &pendingErrors); err != nil {
		return nil, err
	}
	errorSet := make(map[string]struct{}, len(pendingErrors))
	for _, e := range pendingErrors {
		errorSet[e.PlatformID] = struct{}{}
	}

	var result []*ProjectCandidate
	for _, c := range candidates {
		if _, hasError := errorSet[c.PlatformID]; hasError {
			continue
		}
		e, found := existingMap[c.PlatformID]
		if !found || e.PlatformUpdatedAt != c.PlatformUpdatedAt {
			c.ExistingID = e.ID
			result = append(result, c)
		}
	}

	return result, nil
}

// --- API / HTTP ---

func newCandidatesBody(page int) map[string]any {
	return map[string]any{
		"keywords": []any{},
		"projectsFilter": map[string]any{
			"remotePreference":  []any{},
			"city":              []any{},
			"county":            []any{},
			"country":           []any{},
			"projectStart":      []any{},
			"projectDuration":   []any{},
			"lastUpdate":        []any{},
			"includeExclude":    []any{},
			"typeOfContract":    []any{},
			"suggestedTerms":    []any{},
			"profession":        []any{},
			"lastChangedFilter": map[string]any{"filterSectionId": nil, "filterItemId": nil},
		},
		"pagination":    map[string]any{"currentPage": page, "pageSize": candidatesPageSize, "sortBy": "default", "asc": false},
		"category":      "",
		"locale":        "de-DE",
		"searchAgentId": nil,
	}
}

func fetchCandidatesPage(ctx context.Context, s *Session, token string, page int) ([]*ProjectCandidate, error) {
	var apiResp struct {
		Projects apiProjects `json:"projects"`
	}

	client := ownhttp.New(projectsSearchURL, map[string]string{"Authorization": "Bearer " + token})
	if err := client.Post(ctx, "", ownhttp.Request{
		Cookies: s.Cookies(),
		Body:    newCandidatesBody(page),
		Result:  &apiResp,
	}); err != nil {
		return nil, fmt.Errorf("fetch candidates page %d: %w", page, err)
	}

	return apiResp.Projects.toCandidates(), nil
}

// --- API response types ---

type apiProjects []apiProject

func (ps apiProjects) toCandidates() []*ProjectCandidate {
	candidates := make([]*ProjectCandidate, 0, len(ps))
	for _, p := range ps {
		candidates = append(candidates, p.toCandidate())
	}
	return candidates
}

type apiProject struct {
	ID           string   `json:"id"`
	ProjectTitle string   `json:"projectTitle"`
	CompanyName  string   `json:"companyName"`
	CompanyID    string   `json:"companyId"`
	CompanyLogo  []string `json:"companyLogo"`
	SkillTags    []struct {
		SkillName string `json:"skillName"`
	} `json:"skillTags"`
	ProjectStartDate string `json:"projectStartDate"`
	Locations        []struct {
		City    string `json:"city"`
		County  string `json:"county"`
		Country string `json:"country"`
	} `json:"locations"`
	Remote            string `json:"remote"`
	LastUpdate        string `json:"lastUpdate"`
	InsightApplicants struct {
		Link struct {
			URL string `json:"url"`
		} `json:"link"`
	} `json:"insightApplicants"`
	LinkToDetail string `json:"linkToDetail"`
}

func (p *apiProject) toCandidate() *ProjectCandidate {
	skills := make([]string, 0, len(p.SkillTags))
	for _, t := range p.SkillTags {
		skills = append(skills, t.SkillName)
	}

	locations := make([]string, 0, len(p.Locations))
	for _, l := range p.Locations {
		parts := []string{}
		if l.City != "" {
			parts = append(parts, l.City)
		}
		if l.County != "" {
			parts = append(parts, l.County)
		}
		if l.Country != "" {
			parts = append(parts, l.Country)
		}
		if len(parts) > 0 {
			locations = append(locations, strings.Join(parts, ", "))
		}
	}

	logo := ""
	if len(p.CompanyLogo) > 0 {
		logo = p.CompanyLogo[0]
	}

	return &ProjectCandidate{
		PlatformID:        p.ID,
		URL:               p.LinkToDetail,
		Source:            Source,
		Title:             p.ProjectTitle,
		Company:           p.CompanyName,
		CompanyID:         p.CompanyID,
		CompanyLogo:       logo,
		Skills:            skills,
		StartDate:         p.ProjectStartDate,
		Location:          locations,
		Remote:            strings.EqualFold(p.Remote, "remote"),
		PlatformUpdatedAt: p.LastUpdate,
		ScrapedAt:         time.Now(),
	}
}
