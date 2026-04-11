package signal

import (
	"testing"
)

func TestLoadAdminConfig(t *testing.T) {
	tests := []struct {
		name      string
		envValue  string
		wantList  []string
		wantEmpty bool
	}{
		{
			name:      "unset env yields empty config",
			envValue:  "",
			wantList:  nil,
			wantEmpty: true,
		},
		{
			name:      "single email",
			envValue:  "helmer@hblabs.co",
			wantList:  []string{"helmer@hblabs.co"},
			wantEmpty: false,
		},
		{
			name:      "multiple emails comma-separated",
			envValue:  "helmer@hblabs.co,ops@hblabs.co",
			wantList:  []string{"helmer@hblabs.co", "ops@hblabs.co"},
			wantEmpty: false,
		},
		{
			name:      "whitespace around emails is trimmed",
			envValue:  "  helmer@hblabs.co , ops@hblabs.co  ",
			wantList:  []string{"helmer@hblabs.co", "ops@hblabs.co"},
			wantEmpty: false,
		},
		{
			name:      "uppercase normalized to lowercase",
			envValue:  "Helmer@HBLabs.CO,OPS@hblabs.co",
			wantList:  []string{"helmer@hblabs.co", "ops@hblabs.co"},
			wantEmpty: false,
		},
		{
			name:      "duplicates collapsed (case-insensitive)",
			envValue:  "helmer@hblabs.co,HELMER@hblabs.co,ops@hblabs.co",
			wantList:  []string{"helmer@hblabs.co", "ops@hblabs.co"},
			wantEmpty: false,
		},
		{
			name:      "empty entries between commas are skipped",
			envValue:  "helmer@hblabs.co,,ops@hblabs.co,",
			wantList:  []string{"helmer@hblabs.co", "ops@hblabs.co"},
			wantEmpty: false,
		},
		{
			name:      "only whitespace entries yield empty config",
			envValue:  " , , ",
			wantList:  nil,
			wantEmpty: true,
		},
		{
			name:      "preserves declared order",
			envValue:  "z@x.io,a@x.io,m@x.io",
			wantList:  []string{"z@x.io", "a@x.io", "m@x.io"},
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ADMIN_EMAILS", tt.envValue)
			cfg := LoadAdminConfig()

			if got := cfg.Empty(); got != tt.wantEmpty {
				t.Errorf("Empty() = %v, want %v", got, tt.wantEmpty)
			}

			got := cfg.List()
			if !equalStringSlices(got, tt.wantList) {
				t.Errorf("List() = %v, want %v", got, tt.wantList)
			}
		})
	}
}

func TestIsAdmin(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "helmer@hblabs.co, OPS@hblabs.co")
	cfg := LoadAdminConfig()

	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{name: "exact match", email: "helmer@hblabs.co", want: true},
		{name: "case-insensitive match", email: "Helmer@HBLabs.CO", want: true},
		{name: "uppercase env entry matches lowercase query", email: "ops@hblabs.co", want: true},
		{name: "trims whitespace before lookup", email: "  ops@hblabs.co  ", want: true},
		{name: "non-admin email", email: "user@example.com", want: false},
		{name: "empty string", email: "", want: false},
		{name: "whitespace only", email: "   ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.IsAdmin(tt.email); got != tt.want {
				t.Errorf("IsAdmin(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestIsAdminEmptyConfig(t *testing.T) {
	t.Setenv("ADMIN_EMAILS", "")
	cfg := LoadAdminConfig()

	if cfg.IsAdmin("anything@anywhere.com") {
		t.Error("empty config should never report admin")
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
