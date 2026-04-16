package solcomde

import (
	"context"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"

	"hblabs.co/falcon/modules/platformkit"
)

// rssResponse is the top-level RSS XML structure.
type rssResponse struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

// rssItem maps a single <item> from the SOLCOM RSS feed.
type rssItem struct {
	Title       string `xml:"title"`
	Description string `xml:"description"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
}

var feedClient = &http.Client{Timeout: 30 * time.Second}

// fetchFeed downloads and parses the RSS feed, returning all items.
func fetchFeed(ctx context.Context) ([]rssItem, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", platformkit.FalconUserAgent)

	resp, err := feedClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, platformkit.ErrorFromStatus(resp.StatusCode, feedURL, nil)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var rss rssResponse
	if err := xml.Unmarshal(body, &rss); err != nil {
		return nil, err
	}
	return rss.Channel.Items, nil
}

// itemToCandidate converts an RSS item to a ProjectCandidate by parsing
// the structured metadata footer in the description CDATA.
func itemToCandidate(item rssItem) *ProjectCandidate {
	title := strings.TrimSpace(item.Title)
	link := cleanCDATA(item.Link)
	pubDate := parsePubDate(cleanCDATA(item.PubDate))
	desc := cleanCDATA(item.Description)

	// Parse the metadata footer: key-value pairs separated by <br/><br/>
	// at the end of the description CDATA.
	meta := parseDescriptionMeta(desc)

	platformID := meta["Projekt-Nr."]
	if platformID == "" {
		return nil
	}

	location := meta["Einsatzort"]
	remote := strings.Contains(strings.ToLower(location), "remote")

	// Strip the metadata footer from the description so we keep only the
	// human-readable project text.
	cleanDesc := stripMetaFooter(desc)

	return &ProjectCandidate{
		PlatformID:  platformID,
		URL:         link,
		Source:      Source,
		Title:       title,
		Location:    location,
		Remote:      remote,
		StartDate:   parseStartDate(meta["Start"]),
		Duration:    meta["Dauer"],
		JobType:     meta["Stellentyp"],
		Description: cleanHTML(cleanDesc),
		PublishedAt: pubDate,
		ScrapedAt:   time.Now(),
	}
}

// parseDescriptionMeta extracts key-value pairs from the footer of the
// description. The footer has the form:
//
//	Projekt-Nr.: <br/>105381<br/><br/>Stellentyp: <br/>freiberuflich<br/><br/>...
//
// Keys end with ":" and values are between the next <br/> and <br/><br/>.
func parseDescriptionMeta(desc string) map[string]string {
	meta := make(map[string]string)

	// The metadata block starts after "Zusätzliche Informationen:" or is the
	// last line of the description. We look for the key-value pattern.
	// Split by <br/><br/> to get chunks, then parse each chunk.
	chunks := strings.Split(desc, "<br/><br/>")
	for _, chunk := range chunks {
		// Each metadata chunk looks like: "Key: <br/>Value"
		parts := strings.SplitN(chunk, ": <br/>", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		// Only recognized metadata keys
		switch key {
		case "Projekt-Nr.", "Stellentyp", "Einsatzort", "Start", "Dauer":
			meta[key] = value
		}
	}
	return meta
}

// stripMetaFooter removes the structured metadata from the end of the
// description, leaving only the project text.
func stripMetaFooter(desc string) string {
	// The metadata starts with "Projekt-Nr.:" — find it and trim everything after.
	idx := strings.Index(desc, "Projekt-Nr.: <br/>")
	if idx > 0 {
		desc = desc[:idx]
	}
	// Also strip the "Zusätzliche Informationen:" boilerplate if present.
	idx = strings.LastIndex(desc, "Zusätzliche Informationen:")
	if idx > 0 {
		desc = desc[:idx]
	}
	return strings.TrimSpace(desc)
}

// cleanCDATA trims whitespace and CDATA artifacts from XML text content.
func cleanCDATA(s string) string {
	return strings.TrimSpace(s)
}

// cleanHTML strips <br/> and <br /> tags and collapses whitespace for a
// readable plain-text representation of the description.
func cleanHTML(s string) string {
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br>", "\n")
	// Collapse multiple newlines
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

// parsePubDate parses the RSS pubDate which uses a non-standard timezone
// format: "Mon, 13 Apr 2026 00:00:00 Europe/Berlin". Falls back to the
// raw string if parsing fails.
func parsePubDate(s string) string {
	s = strings.TrimSpace(s)
	// Try stripping the timezone name and parsing the rest
	// "Mon, 13 Apr 2026 00:00:00 Europe/Berlin" → "Mon, 13 Apr 2026 00:00:00"
	if idx := strings.LastIndex(s, " "); idx > 0 {
		datePart := s[:idx]
		if t, err := time.Parse("Mon, 02 Jan 2006 15:04:05", datePart); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return s
}

// parseStartDate normalizes a solcom.de start-date string into one of
// the closed canonical shapes shared across falcon-scout platforms:
//
//   - ""             — empty or unparseable
//   - "ab sofort"    — ASAP / immediate variants
//   - "YYYY-MM-DD"   — full calendar date
//
// The solcom feed is particularly messy: many entries are compound
// ("04.05.2026/ spät. 25.05.2026", "asap, spätestens 15.05.2026"),
// mix formats, or give a month+year instead of a full date. Strategy:
//
//  1. Trim + immediate-keyword check.
//  2. Strip a leading German "latest" prefix ("spät. 15.04.2026" uses
//     the date, since it's the only signal we have).
//  3. Compound split: take the *earliest* (first) component when the
//     string contains "/", ",", " spätestens ", " spät.", " spät:",
//     or " oder ". Recurse on that component.
//  4. Try all parse paths: full DD.MM.YYYY, 2-digit year (01.05.26),
//     compact DDMMYYYY (20032026), canonical YYYY-MM-DD,
//     "Anfang|Mitte|Ende <Month>", "<Month> YYYY" or "<Month>/<Month> YYYY".
//  5. Strip German "earliest-start" prefix ("frühster Start in …",
//     "im …") and recurse.
//  6. Drop to "".
func parseStartDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if platformkit.IsImmediateStart(s) {
		return platformkit.CanonicalImmediateStart
	}

	// Strip a leading "latest" prefix — the remaining date is our best signal.
	if stripped, ok := stripLatestPrefix(s); ok {
		return parseStartDate(stripped)
	}

	// Compound: take the earliest piece when the string has a separator.
	if early := extractEarlyStart(s); early != s {
		return parseStartDate(early)
	}

	// European dotted date (strict, lenient, day-clamp recovery).
	if out, ok := platformkit.ParseEuropeanDate(s, "."); ok {
		return out
	}
	// 2-digit year: "01.05.26" → 2026-05-01.
	if out, ok := platformkit.ParseEuropeanDate2DigitYear(s, "."); ok {
		return out
	}
	// Compact DDMMYYYY (8 digits): "20032026" → 2026-03-20.
	if out, ok := platformkit.ParseCompactDate(s); ok {
		return out
	}
	// Already canonical ISO YYYY-MM-DD.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02")
	}
	// German month phrases: "Ende April", "Mitte Mai", "Anfang Juni".
	if out, ok := platformkit.ParseGermanMonthPhrase(s); ok {
		return out
	}
	// German month + year text: "Mai 2026", "April/Mai 2026", "Juli 26".
	if out, ok := platformkit.ParseGermanMonthYear(s); ok {
		return out
	}

	// Strip a leading "earliest" prefix and retry ("im April 2026",
	// "frühster Start in Mai", "ab April").
	if stripped, ok := stripEarliestPrefix(s); ok {
		return parseStartDate(stripped)
	}

	return ""
}

// extractEarlyStart splits s on the first "compound" marker (slash,
// comma, " spätestens ", " spät.", " spät:", " oder ") and returns the
// trimmed portion before that marker. If no marker is present, returns
// s unchanged. Used to pick the *earliest* start date from an entry
// like "04.05.2026/ spät. 25.05.2026".
func extractEarlyStart(s string) string {
	lower := strings.ToLower(s)
	markers := []string{
		" spätestens ",
		" spät.",
		" spät:",
		" oder ",
		"/",
		",",
	}
	earliest := -1
	for _, m := range markers {
		if idx := strings.Index(lower, m); idx >= 0 {
			if earliest < 0 || idx < earliest {
				earliest = idx
			}
		}
	}
	if earliest > 0 {
		return strings.TrimSpace(s[:earliest])
	}
	return s
}

// stripLatestPrefix removes a leading German "latest" phrase so that a
// lone "spät. 15.04.2026" still yields a date (the only date the API
// gave us). Returns (rest, true) when a prefix matched.
func stripLatestPrefix(s string) (string, bool) {
	lower := strings.ToLower(s)
	for _, p := range []string{"spätestens zum ", "spätestens ", "spät.:", "spät. ", "spät:", "spät: "} {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(s[len(p):]), true
		}
	}
	return s, false
}

// stripEarliestPrefix removes a leading German "earliest start" phrase
// so "im April 2026" / "frühster Start in Mai" / "ab April" can be
// re-parsed as a plain month expression. Returns (rest, true) when a
// prefix matched.
func stripEarliestPrefix(s string) (string, bool) {
	lower := strings.ToLower(s)
	for _, p := range []string{"frühster start in ", "im ", "ab dem ", "ab "} {
		if strings.HasPrefix(lower, p) {
			return strings.TrimSpace(s[len(p):]), true
		}
	}
	return s, false
}
