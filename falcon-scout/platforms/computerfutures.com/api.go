package computerfuturescom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hblabs.co/falcon/scout/platformkit"
)

var apiClient = &http.Client{Timeout: 30 * time.Second}

// apiResponse maps the top-level JSON response from sthree search API.
type apiResponse struct {
	Result struct {
		ResultSize int         `json:"resultSize"`
		ResultFrom int         `json:"resultFrom"`
		Hits       int         `json:"hits"`
		Results    []apiResult `json:"results"`
	} `json:"result"`
}

// apiResult maps a single item in the results array. Only the fields we need
// are mapped; the rest (suggest, geoPoint, etc.) are ignored.
type apiResult struct {
	IndexID          string   `json:"indexId"`
	JobReference     string   `json:"jobReference"`
	Title            string   `json:"title"`
	Slug             string   `json:"slug"`
	Description      string   `json:"description"`
	PostDate         string   `json:"postDate"`
	ExpiryDate       string   `json:"expiryDate"`
	JobType          string   `json:"jobType"`
	StartDate        string   `json:"startDate"`
	Duration         string   `json:"duration"`
	Location         string   `json:"location"`
	City             string   `json:"city"`
	Country          string   `json:"country"`
	RemoteAvailable  bool     `json:"remoteWorkingAvailable"`
	Industry         []string `json:"industry"`
	SalaryText       string   `json:"salaryText"`
	SalaryFrom       float64  `json:"salaryFrom"`
	SalaryTo         float64  `json:"salaryTo"`
	SalaryCurrency   string   `json:"salaryCurrency"`
	SalaryPer        string   `json:"salaryPer"`
	SalaryBenefits   string   `json:"salaryBenefits"`
	SalaryHidden     bool     `json:"salaryHidden"`
	Skills           string   `json:"skills"`
	ContactName      string   `json:"contactName"`
	ContactEmail     string   `json:"contactEmail"`
	ApplicationEmail string   `json:"applicationEmail"`
	LastUpdated      string   `json:"lastUpdated"`
	ApplyURL         string   `json:"applyURL"`
}

// searchRequest is the body sent to the sthree search API.
type searchRequest struct {
	Keywords      string   `json:"keywords"`
	Type          []string `json:"type"`
	Industry      string   `json:"industry"`
	City          string   `json:"city"`
	Country       []string `json:"country"`
	RemoteWorking bool     `json:"remoteWorking"`
	ResultFrom    int      `json:"resultFrom"`
	ResultPage    int      `json:"resultPage"`
	ResultSize    int      `json:"resultSize"`
	Language      string   `json:"language"`
	BrandCode     string   `json:"brandCode"`
}

// fetchPage fetches a single page of results from the sthree API.
// Returns the results, total hit count, and any error.
func fetchPage(ctx context.Context, from int) ([]apiResult, int, error) {
	body := searchRequest{
		Keywords:      "",
		Type:          []string{"Freelance"},
		Industry:      "",
		City:          "",
		Country:       []string{"Deutschland", "Schweiz", "Österreich"},
		RemoteWorking: false,
		ResultFrom:    from,
		ResultPage:    0,
		ResultSize:    pageSize,
		Language:      "de-de",
		BrandCode:     "CF",
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", "Abp.Localization.CultureName=en")
	req.Header.Set("User-Agent", platformkit.FalconUserAgent)

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, platformkit.ErrorFromStatus(resp.StatusCode, apiURL, nil)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return nil, 0, fmt.Errorf("unmarshal api response: %w", err)
	}

	return apiResp.Result.Results, apiResp.Result.Hits, nil
}

// resultToCandidate converts an API result to a ProjectCandidate.
// Skips non-freelance results as a safety check — the API filter already
// requests type=Freelance but we don't trust it blindly.
func resultToCandidate(r apiResult) *ProjectCandidate {
	if r.JobReference == "" {
		return nil
	}
	if !strings.EqualFold(r.JobType, "Freelance") {
		return nil
	}

	postDate := parseAPIDate(r.PostDate)

	rateText := r.SalaryText
	if r.SalaryHidden || rateText == "" {
		rateText = "Auf Anfrage"
	}

	return &ProjectCandidate{
		PlatformID:       r.JobReference,
		URL:              r.ApplyURL,
		Source:           Source,
		Title:            r.Title,
		Description:      r.Description,
		Location:         r.Location,
		City:             r.City,
		Country:          r.Country,
		Remote:           r.RemoteAvailable,
		StartDate:        parseStartDate(r.StartDate),
		EndDate:          parseAPIDate(r.ExpiryDate),
		Duration:         orDefault(r.Duration, ""),
		Industry:         r.Industry,
		Skills:           parseSkills(r.Skills),
		PostedAt:         postDate,
		SalaryText:       rateText,
		SalaryFrom:       r.SalaryFrom,
		SalaryTo:         r.SalaryTo,
		SalaryCurrency:   r.SalaryCurrency,
		SalaryPer:        r.SalaryPer,
		SalaryBenefits:   r.SalaryBenefits,
		ContactName:      r.ContactName,
		ContactEmail:     r.ContactEmail,
		ApplicationEmail: r.ApplicationEmail,
		LastUpdated:      parseLastUpdated(r.LastUpdated),
		ScrapedAt:        time.Now(),
	}
}

// parseAPIDate extracts YYYY-MM-DD from an ISO 8601 timestamp like
// "2026-04-14T08:58:49.7051442Z".
func parseAPIDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10] // "2026-04-14"
	}
	return s
}

// parseStartDate normalizes the computerfutures startDate into one of
// a closed set of canonical shapes:
//
//   - ""            — empty or unparseable
//   - "ab sofort"   — ASAP / immediate variants (see platformkit.IsImmediateStart)
//   - "YYYY-MM-DD"  — calendar date
//
// Supported inputs observed in the real feed:
//
//	""              → ""
//	"ASAP"          → "ab sofort"
//	"01.03.2026"    → "2026-03-01"   (DD.MM.YYYY)
//	"01/06/2026"    → "2026-06-01"   (DD/MM/YYYY)
//	"15/03/2026"    → "2026-03-15"
//	"1.7.2026"      → "2026-07-01"   (D.M.YYYY lenient)
//	"1/7/2026"      → "2026-07-01"   (D/M/YYYY lenient)
//
// Calendar-invalid dates ("31.04.2026", "31/04/2026") are recovered by
// clamping the day to the last valid day of the month. Keyword
// normalization and European-date parsing are shared across platforms
// via platformkit.
func parseStartDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if platformkit.IsImmediateStart(s) {
		return platformkit.CanonicalImmediateStart
	}
	for _, sep := range []string{".", "/"} {
		if out, ok := platformkit.ParseEuropeanDate(s, sep); ok {
			return out
		}
	}
	return ""
}

// parseSkills splits a comma-or-semicolon-separated skills string into a
// cleaned []string. Returns nil if the input is empty.
func parseSkills(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Normalize separator: some use ";", some use ","
	s = strings.ReplaceAll(s, ";", ",")
	parts := strings.Split(s, ",")
	var skills []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			skills = append(skills, p)
		}
	}
	return skills
}

// parseLastUpdated converts the relative "Updated: X hours/days ago" string
// from the API into a canonical YYYY-MM-DD date by subtracting the offset
// from now. Returns the raw string if parsing fails.
func parseLastUpdated(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "Updated: ")
	s = strings.TrimSuffix(s, " ago")
	// "5 hours", "1 day", "56 days"
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return s
	}
	var n int
	if _, err := fmt.Sscanf(parts[0], "%d", &n); err != nil || n < 0 {
		return s
	}
	unit := parts[1]
	now := time.Now().UTC()
	var t time.Time
	switch {
	case strings.HasPrefix(unit, "second"):
		t = now.Add(-time.Duration(n) * time.Second)
	case strings.HasPrefix(unit, "minute"):
		t = now.Add(-time.Duration(n) * time.Minute)
	case strings.HasPrefix(unit, "hour"):
		t = now.Add(-time.Duration(n) * time.Hour)
	case strings.HasPrefix(unit, "day"):
		t = now.AddDate(0, 0, -n)
	case strings.HasPrefix(unit, "week"):
		t = now.AddDate(0, 0, -n*7)
	case strings.HasPrefix(unit, "month"):
		t = now.AddDate(0, -n, 0)
	case strings.HasPrefix(unit, "year"):
		t = now.AddDate(-n, 0, 0)
	default:
		return s
	}
	return t.Format(time.RFC3339)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
