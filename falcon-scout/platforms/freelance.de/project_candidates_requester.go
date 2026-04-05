package freelancede

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
	"hblabs.co/falcon/common/system"
)

const candidatesPageSize = 25

// collectNewCandidates paginates the API (candidatesPageSize per page) and stops
// as soon as a page contains at least one already-known candidate — at that point
// all newer projects have been collected. Returns only candidates that need a
// full detail fetch (new or updated), ordered oldest-first after Reverse.
func collectNewCandidates(ctx context.Context) ([]*ProjectCandidate, error) {
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

		platformIDs := make([]string, n)
		for i, c := range candidates {
			platformIDs[i] = c.PlatformID
		}

		var existing []models.PersistedProject
		if err := system.GetStorage().GetManyByField(ctx, constants.MongoProjectsCollection, "platform_id", platformIDs, &existing); err != nil {
			return nil, err
		}

		existingMap := make(map[string]models.PersistedProject, len(existing))
		for _, p := range existing {
			existingMap[p.PlatformID] = p
		}

		newOnPage := 0
		for _, c := range candidates {
			e, found := existingMap[c.PlatformID]
			if !found || e.PlatformUpdatedAt != c.PlatformUpdatedAt {
				c.ExistingID = e.ID
				all = append(all, c)
				newOnPage++
			}
		}

		getLogger().Debugf("page %d: %d/%d new", page, newOnPage, n)

		// Stop when we hit overlap (some known candidates) or the last page.
		if newOnPage < n || n < candidatesPageSize {
			break
		}
	}

	return all, nil
}

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

// fetchCandidatesPage fetches a single page of project candidates from the API.
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

type apiProjects []apiProject

func (ps apiProjects) toCandidates() []*ProjectCandidate {
	candidates := make([]*ProjectCandidate, 0, len(ps))
	for _, p := range ps {
		candidates = append(candidates, p.toCandidate())
	}
	return candidates
}

// apiProject maps the fields returned by /api/ui/projects/search.
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
