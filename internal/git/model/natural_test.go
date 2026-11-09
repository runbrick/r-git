package model

import "testing"

func TestNaturalCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"a", "a", 0},
		{"a", "b", -1},
		{"b", "a", 1},
		{"", "a", -1},
		{"a", "", 1},
		{"", "", 0},
		// Natural numeric order: feature2 < feature10.
		{"feature2", "feature10", -1},
		{"feature10", "feature2", 1},
		{"a2", "a10", -1},
		{"a10", "a10", 0},
		// More significant digits wins, even if the other text differs.
		{"a99", "a100", -1},
		// Leading zeros: equal numeric value, then the longer spelling is greater.
		{"a01", "a1", 1},
		{"a1", "a01", -1},
		{"a1", "a1", 0},
		// Case-insensitive, with a case-sensitive tie break. Upper case
		// sorts first because 'A' < 'a' in the second pass.
		{"Feature", "feature", -1},
		{"feature", "Feature", 1},
		{"FEATURE", "feature", -1},
		// Space sorts after '#' (the transitivity fix).
		{"a b", "a#b", 1},
		{"a#b", "a b", -1},
		// A trailing longer string is greater.
		{"abc", "abcd", -1},
		// Non-ASCII branch names compare by code point and stay stable.
		{"tesla", "teslá", -1},
	}
	for _, tt := range tests {
		got := NaturalCompare(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("NaturalCompare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestBranchNameSort(t *testing.T) {
	names := []string{"release10", "release2", "release1", "main"}
	want := []string{"main", "release1", "release2", "release10"}
	// Insertion sort so the test does not depend on sort.Slice stability.
	for i := 1; i < len(names); i++ {
		j := i
		for j > 0 && NaturalCompare(names[j], names[j-1]) < 0 {
			names[j], names[j-1] = names[j-1], names[j]
			j--
		}
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("sorted = %v, want %v", names, want)
		}
	}
}
