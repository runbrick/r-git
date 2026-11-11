package diff

import "testing"

// Comparable one-sided/identical cases from LineComparisonMergeUtilTest and
// MergeResolveUtilTest; conflicting choices are application behavior.
func TestThreeWayMerge(t *testing.T) {
	for _, tt := range []struct {
		name, base, left, right, want string
		conflicts                     int
	}{
		{"identical", "a\nb\n", "A\nb\n", "A\nb\n", "A\nb\n", 0},
		{"independent", "a\nb\nc\n", "A\nb\nc\n", "a\nb\nC\n", "A\nb\nC\n", 0},
		{"adjacent", "a\nb\n", "A\nb\n", "a\nB\n", "A\nB\n", 0},
		{"overlap", "a\nb\n", "A\nb\n", "X\nb\n", "a\nb\n", 1},
		{"empty", "", "new\n", "", "new\n", 0},
		{"delete", "a\n", "", "a\n", "", 0},
		{"no-final-newline", "a", "ours", "a", "ours", 0},
		{"crlf", "a\r\nb\r\n", "A\r\nb\r\n", "a\r\nB\r\n", "A\r\nB\r\n", 0},
		{"insert-same-place", "a\n", "x\na\n", "y\na\n", "a\n", 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMerge(tt.base, tt.left, tt.right)
			m.ApplyNonConflicting()
			if got := m.Result(); got != tt.want {
				t.Fatalf("result %q want %q chunks %#v", got, tt.want, m.Chunks)
			}
			if m.Remaining() != tt.conflicts {
				t.Fatalf("remaining %d want %d", m.Remaining(), tt.conflicts)
			}
		})
	}
}

func TestChooseDifferentSides(t *testing.T) {
	m := NewMerge("a\nkeep\nb\n", "ours1\nkeep\nours2\n", "theirs1\nkeep\ntheirs2\n")
	if len(m.Chunks) != 2 {
		t.Fatal(m.Chunks)
	}
	m.Chunks[0].Choice = Right
	m.Chunks[1].Choice = Left
	if got := m.Result(); got != "theirs1\nkeep\nours2\n" {
		t.Fatal(got)
	}
}
