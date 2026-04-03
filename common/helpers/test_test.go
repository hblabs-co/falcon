package helpers

import "testing"

func TestPtr(t *testing.T) {
	v := 3.14
	got := Ptr(v)
	if got == nil {
		t.Fatal("Ptr returned nil")
	}
	if *got != v {
		t.Errorf("*Ptr(%v) = %v, want %v", v, *got, v)
	}
	// must return a new allocation, not the address of the argument
	if got == &v {
		t.Error("Ptr should return a new pointer, not the address of the argument")
	}
}

func TestPtr_Zero(t *testing.T) {
	got := Ptr(0)
	if got == nil {
		t.Fatal("Ptr(0) returned nil")
	}
	if *got != 0 {
		t.Errorf("*Ptr(0) = %v, want 0", *got)
	}
}

// CheckStrings, CheckBool and CheckSkills are test-helper functions that call
// t.Errorf on mismatch.  Below we exercise the "no error" path (values match)
// to confirm they do not incorrectly flag equal inputs as failures.

func TestCheckStrings_Match(t *testing.T) {
	CheckStrings(t, "hello", "hello", "field")
}

func TestCheckBool_Match(t *testing.T) {
	CheckBool(t, true, true, "flag")
	CheckBool(t, false, false, "flag")
}

func TestCheckSkills_Match(t *testing.T) {
	CheckSkills(t, []string{"go", "python"}, []string{"go", "python"})
}

func TestCheckSkills_EmptySlices(t *testing.T) {
	CheckSkills(t, []string{}, []string{})
}
