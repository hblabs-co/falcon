package environment

import "testing"

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		value   string
		want    string
		wantErr bool
	}{
		{"present value", "TEST_KEY", "hello", "hello", false},
		{"trims whitespace", "TEST_KEY", "  hello  ", "hello", false},
		{"trims tabs and newlines", "TEST_KEY", "\t hello \n", "hello", false},
		{"empty value returns error", "TEST_KEY", "", "", true},
		{"only whitespace returns error", "TEST_KEY", "   ", "", true},
		{"unset key returns error", "TEST_KEY_UNSET", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value != "" {
				t.Setenv(tc.key, tc.value)
			}
			got, err := Read(tc.key)
			if (err != nil) != tc.wantErr {
				t.Errorf("Read(%q) error = %v, wantErr %v", tc.key, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("Read(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestReadMany(t *testing.T) {
	tests := []struct {
		name    string
		envs    map[string]string
		keys    []string
		want    []string
		wantErr bool
	}{
		{
			name: "all present",
			envs: map[string]string{"A": "1", "B": "2", "C": "3"},
			keys: []string{"A", "B", "C"},
			want: []string{"1", "2", "3"},
		},
		{
			name:    "one missing",
			envs:    map[string]string{"A": "1", "C": "3"},
			keys:    []string{"A", "B", "C"},
			wantErr: true,
		},
		{
			name:    "all missing",
			envs:    map[string]string{},
			keys:    []string{"X", "Y"},
			wantErr: true,
		},
		{
			name:    "empty value treated as missing",
			envs:    map[string]string{"A": "1", "B": ""},
			keys:    []string{"A", "B"},
			wantErr: true,
		},
		{
			name: "trims whitespace",
			envs: map[string]string{"A": "  hello  ", "B": "\tworld\n"},
			keys: []string{"A", "B"},
			want: []string{"hello", "world"},
		},
		{
			name:    "multiple missing reported together",
			envs:    map[string]string{"A": "1"},
			keys:    []string{"A", "B", "C"},
			wantErr: true,
		},
		{
			name: "no keys returns empty slice",
			envs: map[string]string{},
			keys: []string{},
			want: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envs {
				t.Setenv(k, v)
			}
			got, err := ReadMany(tc.keys...)
			if (err != nil) != tc.wantErr {
				t.Errorf("ReadMany(%v) error = %v, wantErr %v", tc.keys, err, tc.wantErr)
				return
			}
			if !tc.wantErr {
				if len(got) != len(tc.want) {
					t.Errorf("ReadMany(%v) len = %d, want %d", tc.keys, len(got), len(tc.want))
					return
				}
				for i := range tc.want {
					if got[i] != tc.want[i] {
						t.Errorf("ReadMany(%v)[%d] = %q, want %q", tc.keys, i, got[i], tc.want[i])
					}
				}
			}
		})
	}
}

func TestParseFloat32(t *testing.T) {
	tests := []struct {
		name  string
		value string
		def   float32
		want  float32
	}{
		{"valid value", "0.85", 0.5, 0.85},
		{"integer value", "1", 0.5, 1.0},
		{"zero value", "0", 0.5, 0.0},
		{"unset uses default", "", 0.75, 0.75},
		{"only whitespace uses default", "   ", 0.75, 0.75},
		{"invalid string uses default", "abc", 0.75, 0.75},
		{"negative value", "-0.5", 0.0, -0.5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value != "" {
				t.Setenv("TEST_FLOAT", tc.value)
			}
			got := ParseFloat32("TEST_FLOAT", tc.def)
			if got != tc.want {
				t.Errorf("ParseFloat32(%q, %v) = %v, want %v", tc.value, tc.def, got, tc.want)
			}
		})
	}
}

func TestReadOptional(t *testing.T) {
	tests := []struct {
		name       string
		key        string
		value      string
		defaultVal string
		want       string
	}{
		{"present value", "TEST_KEY", "hello", "default", "hello"},
		{"trims whitespace", "TEST_KEY", "  hello  ", "default", "hello"},
		{"empty value returns default", "TEST_KEY", "", "default", "default"},
		{"only whitespace returns default", "TEST_KEY", "   ", "default", "default"},
		{"unset key returns default", "TEST_KEY_UNSET", "", "default", "default"},
		{"empty default", "TEST_KEY_UNSET", "", "", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value != "" {
				t.Setenv(tc.key, tc.value)
			}
			got := ReadOptional(tc.key, tc.defaultVal)
			if got != tc.want {
				t.Errorf("ReadOptional(%q, %q) = %q, want %q", tc.key, tc.defaultVal, got, tc.want)
			}
		})
	}
}
