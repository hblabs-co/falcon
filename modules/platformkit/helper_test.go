package platformkit

import "testing"

// stubItem is a minimal ReversibleItem for testing Order / OrderBy.
type stubItem struct {
	id      string
	total   int
	current int
}

func (s *stubItem) SetTotal(n int)   { s.total = n }
func (s *stubItem) SetCurrent(n int) { s.current = n }

func ids(items []*stubItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.id
	}
	return out
}

func TestOrderBy_NumericDescending(t *testing.T) {
	// Contractor-style numeric IDs: a naive lex sort would put "9000"
	// before "30290". OrderBy must recognize them as integers.
	items := []*stubItem{
		{id: "9000"},
		{id: "30290"},
		{id: "12345"},
		{id: "100"},
	}
	keyFn := func(s *stubItem) string { return s.id }

	OrderBy(&items, keyFn, true)

	want := []string{"30290", "12345", "9000", "100"}
	got := ids(items)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("descending: got %v, want %v", got, want)
			break
		}
	}
	// Total/Current populated correctly.
	if items[0].total != 4 || items[0].current != 1 {
		t.Errorf("first: total=%d current=%d, want 4/1", items[0].total, items[0].current)
	}
	if items[3].current != 4 {
		t.Errorf("last: current=%d, want 4", items[3].current)
	}
}

func TestOrderBy_NumericAscending(t *testing.T) {
	items := []*stubItem{
		{id: "30290"},
		{id: "9000"},
		{id: "100"},
	}
	keyFn := func(s *stubItem) string { return s.id }

	OrderBy(&items, keyFn, false)

	want := []string{"100", "9000", "30290"}
	got := ids(items)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ascending: got %v, want %v", got, want)
			break
		}
	}
}

func TestOrderBy_LexFallbackForNonNumeric(t *testing.T) {
	// redglobal-style alphanumeric IDs: fall back to lexicographic.
	items := []*stubItem{
		{id: "CX7F0rXz"},
		{id: "AB12XYZ"},
		{id: "ZZ999"},
	}
	keyFn := func(s *stubItem) string { return s.id }

	OrderBy(&items, keyFn, true)

	want := []string{"ZZ999", "CX7F0rXz", "AB12XYZ"}
	got := ids(items)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("lex descending: got %v, want %v", got, want)
			break
		}
	}
}

func TestOrderBy_MixedNumericAndNonNumeric(t *testing.T) {
	// When one side isn't numeric, fall back to lex for that pair.
	// Here "30290" < "AB" lexicographically ('3' < 'A').
	items := []*stubItem{
		{id: "AB"},
		{id: "30290"},
	}
	keyFn := func(s *stubItem) string { return s.id }

	OrderBy(&items, keyFn, false)

	// Lex: "30290" < "AB" → ascending puts 30290 first.
	if items[0].id != "30290" || items[1].id != "AB" {
		t.Errorf("mixed ascending: got %v, want [30290 AB]", ids(items))
	}
}

func TestOrderBy_Empty(t *testing.T) {
	var items []*stubItem
	OrderBy(&items, func(s *stubItem) string { return s.id }, true)
	// Shouldn't panic on empty slice.
	if len(items) != 0 {
		t.Errorf("empty slice changed length")
	}
}
