package helpers

import "testing"

func TestNormalizeText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty string", "", ""},
		{"no extra whitespace", "hello world", "hello world"},
		{"leading and trailing spaces", "  hello world  ", "hello world"},
		{"multiple internal spaces", "hello   world", "hello world"},
		{"tab between words", "hello\tworld", "hello world"},
		{"newline between words", "hello\nworld", "hello world"},
		{"mixed whitespace and newlines", "  foo  \n  bar\t baz  ", "foo bar baz"},
		{"only whitespace", "   \t\n  ", ""},
		{"single word", "  word  ", "word"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeText(tc.input)
			if got != tc.want {
				t.Errorf("NormalizeText(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
