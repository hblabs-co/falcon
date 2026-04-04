package helpers

import (
	"testing"
)

func TestReverse(t *testing.T) {
	tests := []struct {
		name  string
		input []int
		want  []int
	}{
		{"empty", []int{}, []int{}},
		{"single", []int{1}, []int{1}},
		{"even length", []int{1, 2, 3, 4}, []int{4, 3, 2, 1}},
		{"odd length", []int{1, 2, 3}, []int{3, 2, 1}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := make([]int, len(tc.input))
			copy(got, tc.input)

			Reverse(&got)

			if len(got) != len(tc.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tc.want))
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("got[%d] = %d, want %d", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestReverse_MutatesOriginal(t *testing.T) {
	s := []string{"a", "b", "c"}
	Reverse(&s)
	if s[0] != "c" || s[1] != "b" || s[2] != "a" {
		t.Errorf("expected in-place mutation, got %v", s)
	}
}
