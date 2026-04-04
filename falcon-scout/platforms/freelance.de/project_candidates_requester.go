package freelancede

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

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
	body, err := json.Marshal(map[string]any{
		"keywords": []any{},
		"projectsFilter": map[string]any{
			"remotePreference": []any{},
			"city":             []any{},
			"county":           []any{},
			"country":          []any{},
			"projectStart":     []any{},
			"projectDuration":  []any{},
			"lastUpdate":       []any{},
			"includeExclude":   []any{},
			"typeOfContract":   []any{},
			"suggestedTerms":   []any{},
			"profession":       []any{},
			"lastChangedFilter": map[string]any{
				"filterSectionId": nil,
				"filterItemId":    nil,
			},
		},
		"pagination": map[string]any{
			"currentPage": 1,
			"pageSize":    "100",
			"sortBy":      "default",
			"asc":         false,
		},
		"category":      "",
		"locale":        "de-DE",
		"searchAgentId": nil,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, projectsSearchURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	for _, c := range s.Cookies() {
		req.AddCookie(c)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrCandidatesUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var apiResp struct {
		Projects []apiProject `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	candidates := make([]*ProjectCandidate, 0, len(apiResp.Projects))
	for _, p := range apiResp.Projects {
		candidates = append(candidates, p.toCandidate())
	}
	return candidates, nil
}

// apiProject maps the fields returned by /api/ui/projects/search.
type apiProject struct {
	ID          string `json:"id"`
	ProjectTitle string `json:"projectTitle"`
	CompanyName  string `json:"companyName"`
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
	Remote     string `json:"remote"`
	LastUpdate string `json:"lastUpdate"`
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
		Remote:            p.Remote == "Remote",
		PlatformUpdatedAt: p.LastUpdate,
		ScrapedAt:         time.Now(),
	}
}
