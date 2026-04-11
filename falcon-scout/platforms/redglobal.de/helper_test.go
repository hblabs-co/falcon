package redglobalde

import (
	"testing"
	"time"
)

func TestParseCandidateDate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want time.Time
	}{
		{
			name: "valid canonical date",
			in:   "2026-03-19",
			want: time.Date(2026, 3, 19, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "leap day",
			in:   "2024-02-29",
			want: time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "empty string returns zero",
			in:   "",
			want: time.Time{},
		},
		{
			name: "german format is rejected (caller must normalize first)",
			in:   "19.03.2026",
			want: time.Time{},
		},
		{
			name: "garbage returns zero",
			in:   "not a date",
			want: time.Time{},
		},
		{
			name: "rfc3339 with time is rejected (parseCandidateDate is strict)",
			in:   "2026-03-19T10:00:00Z",
			want: time.Time{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCandidateDate(tt.in)
			if !got.Equal(tt.want) {
				t.Errorf("parseCandidateDate(%q) = %v, want %v", tt.in, got, tt.want)
			}
			if !got.IsZero() && got.Location() != time.UTC {
				t.Errorf("parseCandidateDate(%q) returned non-UTC location: %v", tt.in, got.Location())
			}
		})
	}
}

func TestParseListingDate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "german DD.MM.YYYY normalized to ISO",
			in:   "19.03.2026",
			want: "2026-03-19",
		},
		{
			name: "single-digit day/month padded",
			in:   "01.04.2026",
			want: "2026-04-01",
		},
		{
			name: "leading and trailing whitespace stripped",
			in:   "  19.03.2026  ",
			want: "2026-03-19",
		},
		{
			name: "empty string returns empty",
			in:   "",
			want: "",
		},
		{
			name: "whitespace-only returns empty",
			in:   "   ",
			want: "",
		},
		{
			name: "unparseable date returns input unchanged (fallback)",
			in:   "Heute",
			want: "Heute",
		},
		{
			name: "already-canonical date returns unchanged (fallback path)",
			in:   "2026-03-19",
			want: "2026-03-19",
		},
		{
			name: "invalid german date returns input unchanged",
			in:   "32.13.2026",
			want: "32.13.2026",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListingDate(tt.in)
			if got != tt.want {
				t.Errorf("parseListingDate(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseJobHref(t *testing.T) {
	s := &Scraper{}

	tests := []struct {
		name           string
		href           string
		wantPlatformID string
		wantSlug       string
	}{
		{
			name:           "standard href",
			href:           "/jobs/job/m365-rpa-programmierer-teilzeit-remote-asap/CX7F0rXz",
			wantPlatformID: "CX7F0rXz",
			wantSlug:       "m365-rpa-programmierer-teilzeit-remote-asap",
		},
		{
			name:           "trailing slash trimmed",
			href:           "/jobs/job/sap-o2c-l2o-integration-consultant/d90Iv7ED/",
			wantPlatformID: "d90Iv7ED",
			wantSlug:       "sap-o2c-l2o-integration-consultant",
		},
		{
			name:           "full URL with domain",
			href:           "https://www.redglobal.de/jobs/job/frontend-developer-7-monate/irblReaB",
			wantPlatformID: "irblReaB",
			wantSlug:       "frontend-developer-7-monate",
		},
		{
			name:           "too few segments returns empty",
			href:           "/jobs/job",
			wantPlatformID: "",
			wantSlug:       "",
		},
		{
			name:           "empty string returns empty",
			href:           "",
			wantPlatformID: "",
			wantSlug:       "",
		},
		{
			name:           "single trailing slash on short href",
			href:           "/",
			wantPlatformID: "",
			wantSlug:       "",
		},
		{
			name:           "deeply nested path takes last two segments",
			href:           "/extra/wrap/jobs/job/some-slug/abcXYZ",
			wantPlatformID: "abcXYZ",
			wantSlug:       "some-slug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPlatformID, gotSlug := s.parseJobHref(tt.href)
			if gotPlatformID != tt.wantPlatformID {
				t.Errorf("parseJobHref(%q) platformID = %q, want %q", tt.href, gotPlatformID, tt.wantPlatformID)
			}
			if gotSlug != tt.wantSlug {
				t.Errorf("parseJobHref(%q) slug = %q, want %q", tt.href, gotSlug, tt.wantSlug)
			}
		})
	}
}
