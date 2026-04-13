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
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FalconBot/1.0; +rss-reader)")

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

// parseStartDate normalizes a start date from the metadata footer.
// Handles "01.05.2026" (DD.MM.YYYY), "asap", "Mai 2026", etc.
// Returns canonical YYYY-MM-DD when possible, otherwise the raw string.
func parseStartDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse("02.01.2006", s); err == nil {
		return t.Format("2006-01-02")
	}
	return s // "asap", "Mai 2026", etc. — keep as-is for the LLM
}
