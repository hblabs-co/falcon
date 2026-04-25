package helpers

import "testing"

func Ptr(f float64) *float64 { return &f }

func CheckStrings(t *testing.T, got, want, field string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", field, got, want)
	}
}

func CheckBool(t *testing.T, got, want bool, field string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", field, got, want)
	}
}

func CheckSkills(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("Skills count: got %d %v, want %d %v", len(got), got, len(want), want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Skills[%d]: got %q, want %q", i, got[i], want[i])
		}
	}
}
