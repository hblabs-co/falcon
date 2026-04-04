package freelancede

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hblabs.co/falcon/common/ownhttp"
)

func NewCandidatesBody() map[string]any {
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
		"pagination":    map[string]any{"currentPage": 1, "pageSize": "100", "sortBy": "default", "asc": false},
		"category":      "",
		"locale":        "de-DE",
		"searchAgentId": nil,
	}
}

// FetchProjectCandidates retrieves project candidates from the freelance.de API.
// It reuses a cached JWT access token, refreshing it only when expired or on 401.
func FetchProjectCandidates(ctx context.Context) ([]*ProjectCandidate, error) {
	s := getSession()

	token, err := s.AccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("get access token: %w", err)
	}

	candidates, err := fetchCandidates(ctx, s, token)
	if err != nil {
		return nil, err
	}

	return candidates, nil
}

func fetchCandidates(ctx context.Context, s *Session, token string) ([]*ProjectCandidate, error) {
	var apiResp struct {
		Projects apiProjects `json:"projects"`
	}

	client := ownhttp.New(projectsSearchURL, map[string]string{"Authorization": "Bearer " + token})
	err := client.Post(ctx, "", ownhttp.Request{
		Cookies: s.Cookies(),
		Body:    NewCandidatesBody(),
		Result:  &apiResp,
	})
	if err != nil {
		return nil, fmt.Errorf("fetch candidates: %w", err)
	}

	res := apiResp.Projects.toCandidates()
	return res, nil
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
		CompanyLogo:       logo,
		Skills:            skills,
		StartDate:         p.ProjectStartDate,
		Location:          locations,
		Remote:            strings.EqualFold(p.Remote, "remote"),
		PlatformUpdatedAt: p.LastUpdate,
		ScrapedAt:         time.Now(),
	}
}
