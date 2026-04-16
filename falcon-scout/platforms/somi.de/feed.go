package somide

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hblabs.co/falcon/modules/platformkit"
)

var apiClient = &http.Client{Timeout: 30 * time.Second}

// apiResponse maps the top-level JSON response.
type apiResponse struct {
	Data struct {
		Jobs  []apiJob `json:"jobs"`
		Total int      `json:"total"`
	} `json:"data"`
}

type apiJob struct {
	ID            string           `json:"id"`
	URL           string           `json:"url"`
	Title         string           `json:"title"`
	Descriptions  []apiDescription `json:"descriptions"`
	Location      *apiLocation     `json:"location"`
	Date          *apiDate         `json:"date"`
	ContactPerson *apiContact      `json:"contactPerson"`
	WorkSchedule  *apiWorkBlock    `json:"workSchedule"`
	JobLocation   *apiWorkBlock    `json:"jobLocation"`
	ContractType  string           `json:"contractType"`
	PostDate      string           `json:"postDate"`
	Company       string           `json:"company"`
	Payment       *apiPayment      `json:"payment"`
}

type apiDescription struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

type apiLocation struct {
	Places      []apiPlace `json:"places"`
	Description string     `json:"description"`
}

type apiPlace struct {
	Street  string  `json:"street"`
	Zip     string  `json:"zip"`
	City    string  `json:"city"`
	Region  string  `json:"region"`
	Country string  `json:"country"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
}

type apiDate struct {
	Start       string `json:"start"`
	End         string `json:"end"`
	Description string `json:"description"`
}

type apiContact struct {
	Name     string `json:"name"`
	Position string `json:"position"`
	Phone    string `json:"phone"`
	Email    string `json:"email"`
	Image    string `json:"image"`
}

type apiWorkBlock struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// apiPayment carries the rate block. Both onsite and remote are optional
// — absent jobs have no `payment` field at all, and within a block the
// API sometimes sends zeroes as the "no data" sentinel.
type apiPayment struct {
	Onsite *apiPaymentBlock `json:"onsite"`
	Remote *apiPaymentBlock `json:"remote"`
}

// apiPaymentBlock holds one rate range. `from`/`to` arrive as either a
// string ("65") or a number (0 when empty) — so we decode them as `any`
// and coerce via rateToFloat.
type apiPaymentBlock struct {
	From        any    `json:"from"`
	To          any    `json:"to"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// fetchPage fetches a single page of jobs. Returns the results, total hits,
// and any error.
func fetchPage(ctx context.Context, page int) ([]apiJob, int, error) {
	url := fmt.Sprintf("%s&page=%d", apiURL, page)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", platformkit.FalconUserAgent)

	resp, err := apiClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, platformkit.ErrorFromStatus(resp.StatusCode, url, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, 0, fmt.Errorf("unmarshal api response: %w", err)
	}

	return apiResp.Data.Jobs, apiResp.Data.Total, nil
}

// jobToCandidate converts an API job to a ProjectCandidate. Returns nil if
// the contractType is not freelance (defensive filter — the API request
// already filters but we double-check).
func jobToCandidate(j apiJob) *ProjectCandidate {
	if !strings.EqualFold(j.ContractType, "freelance") {
		return nil
	}
	if j.ID == "" {
		return nil
	}

	location, remote := buildLocation(j.Location, j.JobLocation)
	startDate := parseSomiDate(getDateStart(j.Date))
	endDate := parseSomiDate(getDateEnd(j.Date))
	duration := buildDuration(j.Date)
	rate := buildRate(j.Payment)

	return &ProjectCandidate{
		PlatformID:   j.ID,
		URL:          baseURL + j.URL,
		Source:       Source,
		Title:        unescapeTitle(j.Title),
		Description:  fuseDescriptions(j.Descriptions),
		Location:     location,
		Remote:       remote,
		StartDate:    startDate,
		EndDate:      endDate,
		Duration:     duration,
		WorkSchedule: getWorkType(j.WorkSchedule),
		JobLocation:  getWorkType(j.JobLocation),
		PostedAt:     parseSomiDate(j.PostDate),

		RateOnsiteFrom:        rate.Onsite.From,
		RateOnsiteTo:          rate.Onsite.To,
		RateOnsiteType:        rate.Onsite.Type,
		RateOnsiteDescription: rate.Onsite.Description,

		RateRemoteFrom:        rate.Remote.From,
		RateRemoteTo:          rate.Remote.To,
		RateRemoteType:        rate.Remote.Type,
		RateRemoteDescription: rate.Remote.Description,

		ContactName:     getContactField(j.ContactPerson, "name"),
		ContactPosition: getContactField(j.ContactPerson, "position"),
		ContactPhone:    getContactField(j.ContactPerson, "phone"),
		ContactEmail:    getContactField(j.ContactPerson, "email"),
		ContactImage:    getContactField(j.ContactPerson, "image"),

		ScrapedAt: time.Now(),
	}
}

// fuseDescriptions merges the array of title+content sections into a single
// HTML block with section headers. The LLM normalizer can then parse it
// naturally as structured content.
func fuseDescriptions(sections []apiDescription) string {
	if len(sections) == 0 {
		return ""
	}
	var parts []string
	for _, s := range sections {
		title := strings.TrimSpace(s.Title)
		content := strings.TrimSpace(s.Content)
		if title != "" {
			parts = append(parts, fmt.Sprintf("<h3>%s</h3>", title))
		}
		if content != "" {
			parts = append(parts, content)
		}
	}
	return strings.Join(parts, "\n")
}

// buildLocation fuses all physical places with the jobLocation type (remote,
// hybrid, onsite). Returns the combined location string and whether remote
// work is available.
//
// "hybrid" is rendered as "remote" in the output — from the freelancer's
// perspective the signal that matters is "can I work from home?", and
// hybrid answers yes. The city is still included so they know where the
// on-site days happen.
//
// Cities whose name duplicates the jobLocation label (e.g. a place literally
// called "Remote" when jobLocation is "remote") are filtered out so we
// don't emit "Frankfurt am Main, Remote, remote".
//
// Example outputs:
//   - "Frankfurt am Main, remote"   (hybrid in API → remote label)
//   - "Bulle, Martigny, Villeneuve (VD), onsite"
//   - "remote"
func buildLocation(loc *apiLocation, jobLoc *apiWorkBlock) (string, bool) {
	jobLocType := strings.TrimSpace(getWorkType(jobLoc))
	remote := strings.EqualFold(jobLocType, "remote") || strings.EqualFold(jobLocType, "hybrid")

	// Collapse "hybrid" into "remote" for the human-readable label.
	label := jobLocType
	if strings.EqualFold(label, "hybrid") {
		label = "remote"
	}

	var cities []string
	if loc != nil {
		for _, p := range loc.Places {
			city := strings.TrimSpace(p.City)
			if city == "" {
				continue
			}
			// Skip city names that duplicate the rendered label.
			if strings.EqualFold(city, label) {
				continue
			}
			cities = append(cities, city)
		}
	}

	var parts []string
	if len(cities) > 0 {
		parts = append(parts, cities...)
	}
	if label != "" {
		parts = append(parts, label)
	}
	if jobLoc != nil {
		if desc := strings.TrimSpace(jobLoc.Description); desc != "" {
			parts = append(parts, desc)
		}
	}

	return strings.Join(parts, ", "), remote
}

// rateResult mirrors the JSON payment block: two sides (onsite/remote),
// nothing derived. Display strings and "primary" numeric views are
// computed on demand via formatRateDisplay / pickPrimaryBlock.
type rateResult struct {
	Onsite rateBlock
	Remote rateBlock
}

// rateBlock is the coerced, typed view of one apiPaymentBlock.
type rateBlock struct {
	From        float64
	To          float64
	Type        string // "hourly" | "daily" | ""
	Description string
}

func readRateBlock(b *apiPaymentBlock) rateBlock {
	if b == nil {
		return rateBlock{}
	}
	return rateBlock{
		From:        rateToFloat(b.From),
		To:          rateToFloat(b.To),
		Type:        normalizeRateType(b.Type),
		Description: strings.TrimSpace(b.Description),
	}
}

func (r rateBlock) HasData() bool {
	return r.From > 0 || r.To > 0 || r.Description != ""
}

func (r rateBlock) Equals(o rateBlock) bool {
	return r.From == o.From &&
		r.To == o.To &&
		r.Type == o.Type &&
		r.Description == o.Description
}

// buildRate extracts the onsite/remote blocks from an apiPayment. Returns
// a zero-valued rateResult when payment is nil or both blocks are empty.
func buildRate(p *apiPayment) rateResult {
	if p == nil {
		return rateResult{}
	}
	return rateResult{
		Onsite: readRateBlock(p.Onsite),
		Remote: readRateBlock(p.Remote),
	}
}

// pickPrimaryBlock chooses which side drives single-value views (e.g. the
// numeric amount filter). jobLocation "remote" takes priority; otherwise
// onsite (the canonical rate in the feed) wins, with remote as fallback.
func pickPrimaryBlock(onsite, remote rateBlock, preferRemote bool) rateBlock {
	if preferRemote && remote.HasData() {
		return remote
	}
	if onsite.HasData() {
		return onsite
	}
	return remote
}

// formatRateDisplay renders the rate as a human-readable string.
//
//   - both blocks empty        → "Auf Anfrage"
//   - blocks present & differ  → "<onsite> (onsite) · <remote> (remote)"
//   - blocks identical or one  → single rendering of the primary
func formatRateDisplay(onsite, remote rateBlock, preferRemote bool) string {
	const currency = "EUR"
	onsiteHas := onsite.HasData()
	remoteHas := remote.HasData()
	if !onsiteHas && !remoteHas {
		return "Auf Anfrage"
	}
	if onsiteHas && remoteHas && !onsite.Equals(remote) {
		return formatRateRaw(onsite.From, onsite.To, onsite.Type, currency) +
			" (onsite) · " +
			formatRateRaw(remote.From, remote.To, remote.Type, currency) +
			" (remote)"
	}
	primary := pickPrimaryBlock(onsite, remote, preferRemote)
	return formatRateRaw(primary.From, primary.To, primary.Type, currency)
}

// rateToFloat coerces the from/to field (which the API sends as either a
// string or a number) into a float. Zero / empty / unparseable → 0.
func rateToFloat(v any) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		if f, err := x.Float64(); err == nil {
			return f
		}
	case string:
		s := strings.TrimSpace(x)
		if s == "" {
			return 0
		}
		// Support European decimal comma ("60,50").
		s = strings.ReplaceAll(s, ",", ".")
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}
	return 0
}

func normalizeRateType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "hourly":
		return "hourly"
	case "daily":
		return "daily"
	}
	return ""
}

// formatRateRaw produces a display string like "60-70 €/h", "ab 65 €/h",
// or "Auf Anfrage". Kept in German to match the platform's audience.
func formatRateRaw(from, to float64, rateType, currency string) string {
	unit := ""
	if currency == "EUR" {
		unit = "€"
	}
	suffix := ""
	switch rateType {
	case "hourly":
		suffix = "/h"
	case "daily":
		suffix = "/Tag"
	}
	sep := ""
	if unit != "" {
		sep = " "
	}
	switch {
	case from > 0 && to > 0 && from != to:
		return fmt.Sprintf("%s-%s%s%s%s", trimAmount(from), trimAmount(to), sep, unit, suffix)
	case from > 0 && to > 0:
		return fmt.Sprintf("%s%s%s%s", trimAmount(from), sep, unit, suffix)
	case from > 0:
		return fmt.Sprintf("ab %s%s%s%s", trimAmount(from), sep, unit, suffix)
	case to > 0:
		return fmt.Sprintf("bis %s%s%s%s", trimAmount(to), sep, unit, suffix)
	}
	return "Auf Anfrage"
}

// trimAmount formats a float without trailing ".0" so "65" prints as "65"
// rather than "65.000000", while "60.5" stays "60.5".
func trimAmount(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// buildDuration produces a human-readable project duration from the API's
// loosely-typed date block. Priority:
//
//  1. If end is already a duration string (contains "Monate", "Jahre",
//     has a dash like "3-6 Monate", etc.) → use it as-is.
//  2. If both start and end are full DD.MM.YYYY dates → compute months
//     between them (e.g. "3 Monate").
//  3. If end is partial (year-only "2028", month-year "01.2027") → return
//     "bis <end>".
//  4. If only start is known → empty.
//
// date.description (e.g. "Mit Option auf Verlängerung") is appended when
// present. workSchedule (fullTime/partTime) is NOT part of duration — that
// lives in the WorkSchedule field on the candidate.
func buildDuration(d *apiDate) string {
	if d == nil {
		return ""
	}
	start := strings.TrimSpace(d.Start)
	end := strings.TrimSpace(d.End)

	result := ""
	switch {
	case isDurationString(end):
		result = end
	case isFullDate(start) && isFullDate(end):
		if months := monthsBetween(start, end); months > 0 {
			result = fmt.Sprintf("%d Monate", months)
		}
	case isFullDate(end) || isPartialDate(end):
		result = "bis " + end
	}

	desc := strings.TrimSpace(d.Description)
	if desc != "" {
		if result != "" {
			return result + " · " + desc
		}
		return desc
	}
	return result
}

// isDurationString returns true if s looks like a duration rather than a
// date. Matches values like "3-6 Monate", "6 Monate + Verlängerung",
// "1 Jahr", etc.
func isDurationString(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "monat") ||
		strings.Contains(lower, "jahr") ||
		strings.Contains(lower, "woche") ||
		strings.Contains(lower, "tag")
}

// isFullDate returns true if s matches DD.MM.YYYY exactly.
func isFullDate(s string) bool {
	_, err := time.Parse("02.01.2006", s)
	return err == nil
}

// isPartialDate returns true if s is a plausible partial date: either a
// four-digit year ("2028") or a month-year ("01.2027"). Used to accept end
// values that aren't full DD.MM.YYYY dates but still convey a deadline.
// Rejects keyword-style values like "asap" / "ab sofort" / "nächstmöglich".
func isPartialDate(s string) bool {
	if len(s) == 4 {
		if _, err := time.Parse("2006", s); err == nil {
			return true
		}
	}
	if _, err := time.Parse("01.2006", s); err == nil {
		return true
	}
	return false
}

// monthsBetween returns the number of whole months between two DD.MM.YYYY
// dates. A leftover tail that's essentially another full month (≥30 days)
// counts as +1 — so 01.05 → 31.07 reports as 3 months, not 2. Returns 0
// if either date is unparseable or if end is before start.
func monthsBetween(start, end string) int {
	s, err := time.Parse("02.01.2006", start)
	if err != nil {
		return 0
	}
	e, err := time.Parse("02.01.2006", end)
	if err != nil {
		return 0
	}
	if e.Before(s) {
		return 0
	}
	months := (e.Year()-s.Year())*12 + int(e.Month()-s.Month())
	if e.Day() < s.Day() {
		months--
	}
	if months < 0 {
		months = 0
	}
	// Round up when the leftover days are essentially another full month.
	anniversary := s.AddDate(0, months, 0)
	remainderDays := int(e.Sub(anniversary).Hours() / 24)
	if remainderDays >= 30 {
		months++
	}
	return months
}

// getWorkType returns the Type of a workSchedule/jobLocation block, or "" if
// the block is null.
func getWorkType(b *apiWorkBlock) string {
	if b == nil {
		return ""
	}
	return b.Type
}

// getDateStart returns date.start or "" if date is null.
func getDateStart(d *apiDate) string {
	if d == nil {
		return ""
	}
	return d.Start
}

func getDateEnd(d *apiDate) string {
	if d == nil {
		return ""
	}
	return d.End
}

// getContactField reads a field from the contactPerson, handling null.
func getContactField(c *apiContact, field string) string {
	if c == nil {
		return ""
	}
	switch field {
	case "name":
		return c.Name
	case "position":
		return c.Position
	case "phone":
		return c.Phone
	case "email":
		return c.Email
	case "image":
		return c.Image
	}
	return ""
}

// Immediate-start keyword normalization is shared across platforms via
// platformkit.CanonicalImmediateStart / platformkit.IsImmediateStart.

// datePrefixes are leading German phrases we strip before parsing so the
// inner value can be recognized. Order matters: longer prefixes first.
var datePrefixes = []string{"ab dem ", "ab "}

// stripDatePrefix removes a leading German prefix that wraps a real date
// value (e.g. "Ab dem 01.03.2026" → "01.03.2026"). Returns (inner, true)
// on match, or (s, false) if no prefix applies. "ab sofort" is handled
// earlier in parseSomiDate, so we never strip into a keyword by mistake.
func stripDatePrefix(s string) (string, bool) {
	lower := strings.ToLower(s)
	for _, p := range datePrefixes {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(s[len(p):]), true
		}
	}
	return s, false
}

// parseSomiDate normalizes a somi.de date into one of a small closed set
// of canonical shapes so downstream (DB, UI, LLM) never has to deal with
// API-side inconsistency. The returned string is one of:
//
//   - ""               — unparseable, empty, duration string, or vague text
//   - "ab sofort"      — any "immediate start" variant
//   - "YYYY-MM-DD"     — full calendar date
//
// Partial inputs are closed out to YYYY-MM-DD by assuming end-of-period:
// a month-year (e.g. "11/2025") becomes the last day of that month
// ("2025-11-30"); a bare year ("2028") becomes Dec 31 ("2028-12-31").
// End-of-period is the sensible interpretation both for deadline/end
// fields and for "starting some time in month X" semantics.
//
// Inputs with a calendar-invalid day are recovered by clamping to the
// last valid day of the month ("31.04.2027" → "2027-04-30",
// "32.02.2024" → "2024-02-29"). Only day-overflow is recovered —
// month=13 or day=0 stay rejected.
//
// Duration strings ("12 Monate", "3-6 Monate", "+ 24 Monate") and truly
// vague text ("langfristig", "ab April") return "" — durations belong
// in the Duration field, not in a date field.
//
// Normalizations applied:
//   - "Ab dem 01.03.2026" / "ab 01.05.2026"  → prefix stripped then parsed
//   - "3.11.2025" (lenient padding)          → "2025-11-03"
//   - "31.04.2027" (calendar-invalid day)    → "2027-04-30"
//   - "32.02.2024" (Feb 32 in leap year)     → "2024-02-29"
//   - "11/2025" (slash month-year)           → "2025-11-30"
//   - "1.2027" (unpadded month)              → "2027-01-31"
//   - "2027-01" (ISO month)                  → "2027-01-31"
//   - "2028" (bare year)                     → "2028-12-31"
//   - "2026-04-14T16:06:38" (ISO timestamp)  → "2026-04-14"
//   - "Ende Mai" (month phrase)              → last day of May, year inferred
//   - "Anfang Juni 2027" / "Mitte April"     → day 1 / day 15, year inferred
func parseSomiDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Immediate-start keywords → canonical "ab sofort".
	if platformkit.IsImmediateStart(s) {
		return platformkit.CanonicalImmediateStart
	}
	// Strip German date-wrapping prefixes ("Ab dem …", "ab …").
	if stripped, ok := stripDatePrefix(s); ok {
		s = stripped
		// Defensive: a keyword could surface after stripping.
		if platformkit.IsImmediateStart(s) {
			return platformkit.CanonicalImmediateStart
		}
	}
	// European dotted date (strict, lenient, day-clamp recovery).
	if out, ok := platformkit.ParseEuropeanDate(s, "."); ok {
		return out
	}
	// ISO timestamp → canonical date.
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t.Format("2006-01-02")
	}
	// Already canonical ISO date (validate, don't just slice).
	if len(s) >= 10 && s[4] == '-' && s[7] == '-' {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t.Format("2006-01-02")
		}
	}
	// YYYY-MM (ISO month) → last day of month.
	if t, err := time.Parse("2006-01", s); err == nil {
		return platformkit.LastDayOfMonth(t).Format("2006-01-02")
	}
	// MM/YYYY or M/YYYY → last day of month.
	if t, err := time.Parse("1/2006", s); err == nil {
		return platformkit.LastDayOfMonth(t).Format("2006-01-02")
	}
	// MM.YYYY or M.YYYY → last day of month.
	if t, err := time.Parse("1.2006", s); err == nil {
		return platformkit.LastDayOfMonth(t).Format("2006-01-02")
	}
	// YYYY → last day of year (Dec 31).
	if len(s) == 4 {
		if t, err := time.Parse("2006", s); err == nil {
			return time.Date(t.Year(), 12, 31, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		}
	}
	// German month phrases: "Ende Mai", "Anfang April", "Mitte Juni 2027".
	if out, ok := platformkit.ParseGermanMonthPhrase(s); ok {
		return out
	}
	// Durations, vague German text, anything else → drop.
	return ""
}

// unescapeTitle handles the HTML entities somi.de encodes in job titles,
// e.g. "Java &amp; Angular" → "Java & Angular".
func unescapeTitle(s string) string {
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return strings.TrimSpace(s)
}
