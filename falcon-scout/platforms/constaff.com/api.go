package constaffcom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hblabs.co/falcon/modules/platformkit"
)

var apiClient = &http.Client{Timeout: 30 * time.Second}

// apiResponse mirrors the top-level JSON from constaff's admin-ajax endpoint.
// `header` / `query` / `type` carry PHP/curl metadata we ignore.
type apiResponse struct {
	Projects []apiProject `json:"projects"`
	Count    int          `json:"count"` // total across all pages
	Type     string       `json:"type"`
}

// apiProject maps a single entry inside `projects`. The API returns dates
// as free-form German text and ClosingDate in Microsoft JSON format.
type apiProject struct {
	Begin               string   `json:"Begin"`       // "April 2026", "Mai 2026"
	End                 string   `json:"End"`         // "12 Monate ++", "8 Months ++" (duration)
	ID                  int      `json:"Id"`          // numeric, e.g. 14274
	DisplayJobmarket    bool     `json:"DisplayJobmarket"`
	JobDescriptionHTML  string   `json:"JobDescriptionHTML"`
	ClosingDate         string   `json:"ClosingDate"` // "/Date(1777485600000+0000)/"
	JobLocation         string   `json:"JobLocation"` // "D5 Köln/hybrid" | "Remote"
	JobTitle            string   `json:"JobTitle"`
	Type                int      `json:"Type"`        // 1 | 3 | 5
	TypeStr             []string `json:"TypeStr"`     // ["Freiberuflich", "Contractor"]
	User                string   `json:"User"`        // "JBA" (initials)
}

// fetchPage fetches a single page of constaff projects. page is 1-indexed.
// Returns the raw results, the total count (across all pages), and any error.
func fetchPage(ctx context.Context, page int) ([]apiProject, int, error) {
	body, contentType, err := buildSearchForm(page)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("User-Agent", platformkit.FalconUserAgent)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, platformkit.ErrorFromStatus(resp.StatusCode, apiURL, nil)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	var out apiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, 0, fmt.Errorf("unmarshal constaff response: %w", err)
	}
	return out.Projects, out.Count, nil
}

// buildSearchForm constructs the multipart/form-data body the PHP backend
// expects. Mirrors the `curl --form` fields from examples/request exactly.
func buildSearchForm(page int) (io.Reader, string, error) {
	buf := &bytes.Buffer{}
	w := multipart.NewWriter(buf)
	fields := [][2]string{
		{"action", "searchProjects"},
		{"contractsAsString[]", "Contracting"},
		{"selectedContracts[]", "5"},
		{"selectedContracts[]", "3"},
		{"selectedContracts[]", "1"},
		{"searchQuery", ""},
		{"page", strconv.Itoa(page)},
	}
	for _, f := range fields {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return nil, "", err
		}
	}
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf, w.FormDataContentType(), nil
}

// projectToCandidate converts an API project to a ProjectCandidate.
// Returns nil when TypeStr doesn't include a freelance label — defensive,
// since the request already filters by Contracting IDs.
func projectToCandidate(p apiProject) *ProjectCandidate {
	label, isANUE, ok := classifyTypeStr(p.TypeStr)
	if !ok {
		return nil
	}
	if p.ID == 0 {
		return nil
	}
	location, remote := buildLocation(p.JobLocation)

	return &ProjectCandidate{
		PlatformID:  strconv.Itoa(p.ID),
		URL:         fmt.Sprintf("%s/?p=%d", baseURL, p.ID), // best-effort detail URL
		Source:      Source,
		Title:       strings.TrimSpace(p.JobTitle),
		Description: strings.TrimSpace(p.JobDescriptionHTML),
		Location:    location,
		Remote:      remote,
		StartDate:   parseBeginDate(p.Begin),
		EndDate:     "", // not provided — End is a duration, not a date
		Duration:    strings.TrimSpace(p.End),
		PostedAt:    "", // no publish date in API; ScrapedAt drives display_updated_at
		TypeLabel:       label,
		IsANUE:          isANUE,
		ContactInitials: strings.TrimSpace(p.User),
		ScrapedAt:       time.Now(),
	}
}

// classifyTypeStr inspects the TypeStr array and returns:
//   - the preferred human label ("Freiberuflich" / "Contractor")
//   - whether ", ANÜ" is present (labor-leasing variant)
//   - ok=false when no known label is found (candidate is dropped)
func classifyTypeStr(ts []string) (label string, isANUE bool, ok bool) {
	for _, s := range ts {
		lower := strings.ToLower(s)
		if strings.Contains(lower, "freiberuflich") || strings.Contains(lower, "contractor") {
			ok = true
			// Prefer the German label when we see both entries.
			if label == "" || strings.Contains(lower, "freiberuflich") {
				label = strings.SplitN(strings.TrimSpace(s), ",", 2)[0]
			}
			if strings.Contains(lower, "anü") {
				isANUE = true
			}
		}
	}
	return label, isANUE, ok
}

// buildLocation cleans a JobLocation string like "D5 Köln/hybrid" into
// "Köln / hybrid" and derives the remote flag. "D<n> " is a region code
// prefix the platform uses internally; we strip it for display.
func buildLocation(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", false
	}
	remote := strings.Contains(strings.ToLower(s), "remote") ||
		strings.Contains(strings.ToLower(s), "hybrid")

	// Strip leading "D<digit(s)> " region code ("D5 Köln/hybrid" → "Köln/hybrid").
	if len(s) > 2 && s[0] == 'D' {
		i := 1
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i > 1 && i < len(s) && s[i] == ' ' {
			s = strings.TrimSpace(s[i+1:])
		}
	}
	// Normalize slash spacing: "Köln/hybrid" → "Köln / hybrid".
	if !strings.Contains(s, " / ") && strings.Contains(s, "/") {
		parts := strings.Split(s, "/")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		s = strings.Join(parts, " / ")
	}
	return s, remote
}

// parseBeginDate normalizes the Begin field ("April 2026", "Mai 2026",
// "asap", etc.) into canonical YYYY-MM-DD or "ab sofort". Falls back to
// "" when nothing parses. Reuses platformkit helpers end-to-end.
func parseBeginDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if platformkit.IsImmediateStart(s) {
		return platformkit.CanonicalImmediateStart
	}
	if out, ok := platformkit.ParseEuropeanDate(s, "."); ok {
		return out
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02")
	}
	if out, ok := platformkit.ParseGermanMonthPhrase(s); ok {
		return out
	}
	if out, ok := platformkit.ParseGermanMonthYear(s); ok {
		return out
	}
	return ""
}

// ─── Contact Directory ──────────────────────────────────────────────

// contactEntry holds the resolved contact info for one team member,
// keyed by their 3-letter initials (e.g. "JBA").
type contactEntry struct {
	Name     string // "Julia Barth"
	Position string // "Expert Candidate Relations"
	Phone    string // "+49 6221 33896-262"
	Email    string // "julia.barth@constaff.com"
	Image    string // "https://www.constaff.com/wp-content/uploads/…/JBA.jpg"
}

// apiAnsprechpartner maps a single entry from the WP REST API endpoint
// /wp-json/wp/v2/ansprechpartner.
type apiAnsprechpartner struct {
	Title struct {
		Rendered string `json:"rendered"` // "JBA" (initials)
	} `json:"title"`
	APName     string          `json:"apname"`
	APPosition string          `json:"apposition"`
	APPhone    string          `json:"apphone"`
	APEmail    string          `json:"apemail"`
	APPicture  json.RawMessage `json:"appicture"` // object {"guid":"…"} or false (PHP)
}

type apiPicture struct {
	GUID string `json:"guid"` // image URL
}

// extractPictureGUID safely parses the appicture field which PHP sends
// as either an object (with guid) or the literal `false` when empty.
func extractPictureGUID(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Fast-reject PHP's `false` / JSON null without allocating.
	if raw[0] != '{' {
		return ""
	}
	var pic apiPicture
	if err := json.Unmarshal(raw, &pic); err != nil {
		return ""
	}
	return strings.TrimSpace(pic.GUID)
}

// fetchContactDirectory fetches the full team directory and returns a
// map from uppercase initials → contactEntry. Called once on Init() and
// refreshed daily by the worker goroutine.
func fetchContactDirectory(ctx context.Context) (map[string]contactEntry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, contactsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", platformkit.FalconUserAgent)

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, platformkit.ErrorFromStatus(resp.StatusCode, contactsURL, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var entries []apiAnsprechpartner
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, fmt.Errorf("unmarshal ansprechpartner: %w", err)
	}

	dir := make(map[string]contactEntry, len(entries))
	for _, e := range entries {
		initials := strings.ToUpper(strings.TrimSpace(e.Title.Rendered))
		if initials == "" {
			continue
		}
		imageURL := extractPictureGUID(e.APPicture)
		dir[initials] = contactEntry{
			Name:     strings.TrimSpace(e.APName),
			Position: strings.TrimSpace(e.APPosition),
			Phone:    strings.TrimSpace(e.APPhone),
			Email:    strings.TrimSpace(e.APEmail),
			Image:    imageURL,
		}
	}
	return dir, nil
}

// ─── Date Parsing ───────────────────────────────────────────────────

// parseMSDate converts a Microsoft JSON date string — "/Date(1776276000000+0000)/"
// — into canonical YYYY-MM-DD. Returns "" when the input doesn't match.
// The trailing "+HHMM" timezone offset is ignored (we only care about the day).
func parseMSDate(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "/Date(") || !strings.HasSuffix(s, ")/") {
		return ""
	}
	inner := s[len("/Date(") : len(s)-len(")/")]
	// Split optional "+HHMM" / "-HHMM" suffix.
	for i := 1; i < len(inner); i++ {
		if inner[i] == '+' || inner[i] == '-' {
			inner = inner[:i]
			break
		}
	}
	ms, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return ""
	}
	return time.UnixMilli(ms).UTC().Format("2006-01-02")
}
