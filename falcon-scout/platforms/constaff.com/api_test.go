package constaffcom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gocolly/colly/v2"
	"hblabs.co/falcon/scout/platformkit"
)

func fixedNow(t *testing.T) {
	t.Helper()
	prev := platformkit.NowFunc
	platformkit.NowFunc = func() time.Time {
		return time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() { platformkit.NowFunc = prev })
}

func TestParseBeginDate(t *testing.T) {
	fixedNow(t)

	tests := []struct {
		input string
		want  string
	}{
		// observed in examples/response.json
		{"April 2026", "2026-04-01"},
		{"Mai 2026", "2026-05-01"},
		// defensive / extra formats
		{"", ""},
		{"asap", "ab sofort"},
		{"Ab sofort", "ab sofort"},
		{"01.05.2026", "2026-05-01"},
		{"2026-04-01", "2026-04-01"},
		{"Ende Mai", "2026-05-31"},
		{"Mitte April", "2026-04-15"},
		{"foobar", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseBeginDate(tt.input); got != tt.want {
				t.Errorf("parseBeginDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseMSDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// observed in examples/response.json
		{"/Date(1776276000000+0000)/", "2026-04-15"},
		{"/Date(1776967200000+0000)/", "2026-04-23"},
		{"/Date(1777485600000+0000)/", "2026-04-29"},
		{"/Date(1778004000000+0000)/", "2026-05-05"},
		// negative offset / zero / garbage
		{"/Date(0)/", "1970-01-01"},
		{"/Date(-1000+0000)/", "1969-12-31"},
		{"", ""},
		{"not a date", ""},
		{"/Date(abc)/", ""},
		{"/Date(1776276000000)/", "2026-04-15"}, // no offset suffix
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseMSDate(tt.input); got != tt.want {
				t.Errorf("parseMSDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildLocation(t *testing.T) {
	tests := []struct {
		raw        string
		wantLoc    string
		wantRemote bool
	}{
		// observed
		{"Remote", "Remote", true},
		{"D1 Berlin / hybrid", "Berlin / hybrid", true},
		{"D5 Köln/hybrid", "Köln / hybrid", true},
		{"D6 Region Voralberg / hybrid", "Region Voralberg / hybrid", true},
		{"D6 St. Wendel / hybrid", "St. Wendel / hybrid", true},
		{"D9 Nürnberg / hybrid", "Nürnberg / hybrid", true},

		// edge cases
		{"", "", false},
		{"Köln", "Köln", false},
		{"D12 Hamburg", "Hamburg", false}, // 2-digit region code
		{"Düsseldorf / onsite", "Düsseldorf / onsite", false},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			gotLoc, gotRemote := buildLocation(tt.raw)
			if gotLoc != tt.wantLoc || gotRemote != tt.wantRemote {
				t.Errorf("buildLocation(%q) = (%q, %v), want (%q, %v)",
					tt.raw, gotLoc, gotRemote, tt.wantLoc, tt.wantRemote)
			}
		})
	}
}

func TestClassifyTypeStr(t *testing.T) {
	tests := []struct {
		name      string
		input     []string
		wantLabel string
		wantANUE  bool
		wantOK    bool
	}{
		{"classic", []string{"Freiberuflich", "Contractor"}, "Freiberuflich", false, true},
		{"ANÜ variant", []string{"Freiberuflich, ANÜ", "Contractor, ANÜ"}, "Freiberuflich", true, true},
		{"contractor only", []string{"Contractor"}, "Contractor", false, true},
		{"empty", []string{}, "", false, false},
		{"unknown", []string{"Festanstellung"}, "", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			label, anue, ok := classifyTypeStr(tt.input)
			if label != tt.wantLabel || anue != tt.wantANUE || ok != tt.wantOK {
				t.Errorf("classifyTypeStr(%v) = (%q, %v, %v), want (%q, %v, %v)",
					tt.input, label, anue, ok, tt.wantLabel, tt.wantANUE, tt.wantOK)
			}
		})
	}
}

func TestExtractPictureGUID(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"object with guid", `{"ID":"4743","guid":"https://www.constaff.com/wp-content/uploads/2025/06/AKH.jpg","post_type":"attachment"}`, "https://www.constaff.com/wp-content/uploads/2025/06/AKH.jpg"},
		{"false (PHP no-picture)", `false`, ""},
		{"null", `null`, ""},
		{"true", `true`, ""},
		{"empty", ``, ""},
		{"empty object", `{}`, ""},
		{"number", `0`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPictureGUID(json.RawMessage(tt.raw))
			if got != tt.want {
				t.Errorf("extractPictureGUID(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestUnmarshalAnsprechpartner(t *testing.T) {
	// Covers both appicture-as-object and appicture-as-false in one unmarshal.
	raw := []byte(`[
		{
			"title": {"rendered": "JBA"},
			"apname": "Julia Barth",
			"apposition": "Expert Candidate Relations",
			"apphone": "+49 6221 33896-262",
			"apemail": "julia.barth@constaff.com",
			"appicture": {"guid": "https://www.constaff.com/wp-content/uploads/JBA.jpg"}
		},
		{
			"title": {"rendered": "XYZ"},
			"apname": "No Photo Person",
			"apposition": "Role",
			"apphone": "+49 123",
			"apemail": "no@photo.com",
			"appicture": false
		}
	]`)
	var entries []apiAnsprechpartner
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	// First: has picture
	if got := extractPictureGUID(entries[0].APPicture); got != "https://www.constaff.com/wp-content/uploads/JBA.jpg" {
		t.Errorf("entry[0] picture = %q, want JBA.jpg URL", got)
	}
	// Second: appicture=false → empty
	if got := extractPictureGUID(entries[1].APPicture); got != "" {
		t.Errorf("entry[1] picture = %q, want empty", got)
	}
}

func TestParseContactFromDetailHTML(t *testing.T) {
	raw, err := os.ReadFile("examples/detail-site.html")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}

	// Feed the raw HTML into a colly HTMLElement by starting a test server.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(raw)
	}))
	defer ts.Close()

	var entry contactEntry
	var found bool

	c := colly.NewCollector()
	c.OnHTML(".projectdetails-details-sidebar-contact", func(e *colly.HTMLElement) {
		found = true
		entry.Name = strings.TrimSpace(e.ChildText(".projectdetails-details-sidebar-contact-p-bold"))
		e.ForEach(".projectdetails-details-sidebar-contact-p", func(_ int, el *colly.HTMLElement) {
			text := strings.Join(strings.Fields(el.Text), " ")
			switch {
			case strings.HasPrefix(text, "Tel.:"):
				entry.Phone = strings.TrimSpace(strings.TrimPrefix(text, "Tel.:"))
			case strings.HasPrefix(text, "Mail:"):
				entry.Email = strings.TrimSpace(strings.TrimPrefix(text, "Mail:"))
			case entry.Position == "":
				entry.Position = text
			}
		})
		entry.Image = e.ChildAttr(".projectdetails-details-sidebar-picture", "src")
	})

	if err := c.Visit(ts.URL); err != nil {
		t.Fatalf("visit: %v", err)
	}
	if !found {
		t.Fatal("contact section not found in example HTML")
	}
	if entry.Name != "Ann-Kathrin Gelb" {
		t.Errorf("Name = %q, want %q", entry.Name, "Ann-Kathrin Gelb")
	}
	if entry.Position != "Expert Candidate Relations" {
		t.Errorf("Position = %q, want %q", entry.Position, "Expert Candidate Relations")
	}
	if entry.Phone != "+49 6221 33896-245" {
		t.Errorf("Phone = %q, want %q", entry.Phone, "+49 6221 33896-245")
	}
	if entry.Email != "ann-kathrin.gelb@constaff.com" {
		t.Errorf("Email = %q, want %q", entry.Email, "ann-kathrin.gelb@constaff.com")
	}
	if entry.Image != "https://www.constaff.com/wp-content/uploads/2025/06/AKH.jpg" {
		t.Errorf("Image = %q, want AKH.jpg URL", entry.Image)
	}
}

// TestUnmarshalRealResponse verifies the apiResponse shape matches the
// actual JSON shape from examples/response.json, and that project
// conversion produces the expected canonical values.
func TestUnmarshalRealResponse(t *testing.T) {
	fixedNow(t)

	raw, err := os.ReadFile("examples/response.json")
	if err != nil {
		t.Fatalf("read example: %v", err)
	}
	var out apiResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Count == 0 {
		t.Errorf("count should be > 0")
	}
	if len(out.Projects) == 0 {
		t.Fatalf("projects should be > 0")
	}
	// Spot-check first project conversion.
	c := projectToCandidate(out.Projects[0])
	if c == nil {
		t.Fatalf("first project → nil candidate")
	}
	if c.PlatformID == "" || c.Title == "" {
		t.Errorf("candidate missing PlatformID or Title: %+v", c)
	}
	if c.StartDate == "" {
		t.Errorf("StartDate should parse from %q", out.Projects[0].Begin)
	}
	// PostedAt intentionally empty — constaff has no publish date,
	// display_updated_at is driven by ScrapedAt.
	if c.PostedAt != "" {
		t.Errorf("PostedAt should be empty (no publish date in API), got %q", c.PostedAt)
	}
	if c.Duration == "" {
		t.Errorf("Duration should be End field non-empty (got %q from %q)",
			c.Duration, out.Projects[0].End)
	}
	if c.TypeLabel == "" {
		t.Errorf("TypeLabel should be set from TypeStr %v", out.Projects[0].TypeStr)
	}
}
